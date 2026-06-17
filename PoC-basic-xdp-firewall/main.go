package main

import (
	"bytes"
	_ "embed"
	"encoding/binary"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
	"gopkg.in/yaml.v3"
	"sort"
	"strings"
)

type ConfigRule struct {
	ID       uint32 `yaml:"id"`
	Priority uint32 `yaml:"priority"`
	SrcCIDR  string `yaml:"src_cidr"`
	DstCIDR  string `yaml:"dst_cidr"`
	DstPort  uint16 `yaml:"dst_port"`
	Proto    string `yaml:"proto"`
	Action   string `yaml:"action"`
}

type Config struct {
	Rules []ConfigRule `yaml:"rules"`
}

// FirewallRule must match the C struct exactly.
type FirewallRule struct {
	SrcIP   uint32
	SrcMask uint32
	DstIP   uint32
	DstMask uint32
	DstPort uint16
	Proto   uint8
	Action  uint8
	RuleID  uint32
}

// PacketEvent is decoded from the raw bytes sent by the BPF program.
type PacketEvent struct {
	SrcIP     uint32
	DstIP     uint32
	SrcPort   uint16
	DstPort   uint16
	Proto     uint8
	Action    uint8
	_         [2]uint8 // Padding
	RuleID    uint32
	Timestamp uint64
}

//go:embed bpf/firewall.bpf.o
var bpfProg []byte

func main() {
	var ifaceName string
	flag.StringVar(&ifaceName, "iface", "", "Network interface to attach the firewall to")
	flag.Parse()

	fmt.Println("=== BPF Firewall PROTOTYPE ===")
	fmt.Println("🔍 Advanced Logging & Scalable Rules (DEFAULT DROP)")

	// Remove memory limits for BPF map allocation
	if err := rlimit.RemoveMemlock(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", err)
	}

	// Load the BPF object file from embedded bytes
	spec, err := ebpf.LoadCollectionSpecFromReader(bytes.NewReader(bpfProg))
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to load embedded BPF spec: %s\n", err)
		os.Exit(1)
	}

	// Create a new BPF collection (programs + maps)
	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s\n", err)
		os.Exit(1)
	}
	defer coll.Close()

	xdpProg := coll.Programs["firewall_filter"]
	eventsMap := coll.Maps["events"]
	rulesMap := coll.Maps["rules"]
	configMap := coll.Maps["config"]

	// Periodic configuration polling
	const configPath = "rules.yaml"
	go func() {
		var lastModTime time.Time
		for {
			info, err := os.Stat(configPath)
			if err != nil {
				if !os.IsNotExist(err) {
					fmt.Fprintf(os.Stderr, "Error stating config file: %v\n", err)
				}
				time.Sleep(5 * time.Second)
				continue
			}

			if info.ModTime().After(lastModTime) {
				fmt.Printf("\n--- Loading Config: %s ---\n", configPath)
				config, err := loadConfig(configPath)
				if err != nil {
					fmt.Fprintf(os.Stderr, "❌ Failed to load config: %v\n", err)
				} else {
					syncRules(rulesMap, configMap, config)
					lastModTime = info.ModTime()
					fmt.Println("--- Config Synced ---")
				}
			}
			time.Sleep(5 * time.Second)
		}
	}()

	if ifaceName == "" {
		// User interface for interface selection
		fmt.Println("\n=== Interfaces ===")
		interfaces, _ := net.Interfaces()
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
				fmt.Printf("  • %s (idx=%d)\n", iface.Name, iface.Index)
			}
		}

		fmt.Print("\nInterface to attach (or press Enter for 'eth0'): ")
		fmt.Scanln(&ifaceName)
		if ifaceName == "" {
			ifaceName = "eth0"
		}
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Interface not found: %s\n", err)
		os.Exit(1)
	}

	// Attach the XDP program to the selected interface
	xdpLink, err := link.AttachXDP(link.XDPOptions{
		Program:   xdpProg,
		Interface: iface.Index,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ XDP: %s\n", err)
		os.Exit(1)
	}
	defer xdpLink.Close()
	fmt.Println("✓ XDP attached to", ifaceName)

	// Start a goroutine to read and display packet events
	go func() {
		perfReader, err := perf.NewReader(eventsMap, os.Getpagesize())
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Perf Reader: %s\n", err)
			return
		}
		defer perfReader.Close()

		fmt.Println("\n--- Monitoring Packets ---")
		for {
			record, err := perfReader.Read()
			if err != nil {
				if err == perf.ErrClosed {
					return
				}
				fmt.Fprintf(os.Stderr, "Read error: %s\n", err)
				continue
			}

			if record.LostSamples != 0 {
				fmt.Printf("Lost %d samples\n", record.LostSamples)
				continue
			}

			// Decode raw bytes into the PacketEvent struct
			var event PacketEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to decode event: %s\n", err)
				continue
			}

			printEvent(event)
		}
	}()

	// Wait for interrupt to exit gracefully
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down...")
}

// parseCIDR: Helper to convert CIDR string to uint32 IP and Mask.
// We return them as host-order uint32 to match how BPF loads be32 into registers on LE hosts.
func parseCIDR(cidrStr string) (uint32, uint32) {
	if cidrStr == "" {
		return 0, 0
	}
	ip, ipNet, err := net.ParseCIDR(cidrStr)
	if err != nil {
		// Fallback to single IP if not CIDR
		ipv4 := net.ParseIP(cidrStr).To4()
		if ipv4 == nil {
			return 0xFFFFFFFF, 0xFFFFFFFF
		}
		// Return IP in LittleEndian (host order for x86)
		return binary.LittleEndian.Uint32(ipv4), 0xFFFFFFFF
	}

	ipv4 := ip.To4()
	if ipv4 == nil {
		return 0xFFFFFFFF, 0xFFFFFFFF
	}

	mask := binary.BigEndian.Uint32(ipNet.Mask)
	// Mask from binary.BigEndian.Uint32 is already in a format where bitwise AND works
	// if the IP is also handled the same way.
	// However, net.IP bytes are [192, 168, 1, 0].
	// binary.LittleEndian.Uint32([192, 168, 1, 0]) -> 0x0001A8C0
	// binary.BigEndian.Uint32([192, 168, 1, 0]) -> 0xC0A80100

	// If BPF does: src_ip & mask
	// On LE: src_ip (from packet) is loaded into register.
	// Packet: [C0, A8, 01, 01]
	// Register: 0x0101A8C0 (Little Endian)

	// So we should use LittleEndian.Uint32 for both to match host register layout.

	maskBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(maskBytes, mask)

	return binary.LittleEndian.Uint32(ipv4), binary.LittleEndian.Uint32(maskBytes)
}

// ipToUint32: Helper to convert IP string to LittleEndian uint32 for BPF map keys.
func ipToUint32(ipStr string) uint32 {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(ip)
}

// htons: Host-to-Network Short (to BigEndian for network field matching)
func htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return binary.LittleEndian.Uint16(b)
}

// printEvent: Formats and displays a packet event.
func printEvent(e PacketEvent) {
	srcIP := make(net.IP, 4)
	binary.LittleEndian.PutUint32(srcIP, e.SrcIP)
	dstIP := make(net.IP, 4)
	binary.LittleEndian.PutUint32(dstIP, e.DstIP)

	actionStr := "✅ PASS"
	if e.Action == 0 {
		actionStr = "❌ DROP"
	}

	protoStr := fmt.Sprintf("PROTO=%d", e.Proto)
	switch e.Proto {
	case 1:
		protoStr = "ICMP"
	case 6:
		protoStr = "TCP"
	case 17:
		protoStr = "UDP"
	}

	// Timestamp is relative to system boot (nanoseconds)
	duration := time.Duration(e.Timestamp) * time.Nanosecond

	fmt.Printf("[%12s] %s | %s | %s:%d -> %s:%d | Rule=%d\n",
		duration.String(), actionStr, protoStr, srcIP, ntohs(e.SrcPort), dstIP, ntohs(e.DstPort), e.RuleID)
}

// ntohs: Network-to-Host Short
func ntohs(i uint16) uint16 {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, i)
	return binary.BigEndian.Uint16(b)
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

func protoToUint8(proto string) uint8 {
	switch strings.ToUpper(proto) {
	case "ICMP":
		return 1
	case "TCP":
		return 6
	case "UDP":
		return 17
	default:
		return 0
	}
}

func syncRules(rulesMap *ebpf.Map, configMap *ebpf.Map, config *Config) {
	// 1. Sort rules by priority
	sort.Slice(config.Rules, func(i, j int) bool {
		return config.Rules[i].Priority < config.Rules[j].Priority
	})

	// 2. Push rules to the Array map
	var i uint32
	for i = 0; i < uint32(len(config.Rules)) && i < 256; i++ {
		cr := config.Rules[i]
		srcIP, srcMask := parseCIDR(cr.SrcCIDR)
		dstIP, dstMask := parseCIDR(cr.DstCIDR)

		rule := FirewallRule{
			SrcIP:   srcIP,
			SrcMask: srcMask,
			DstIP:   dstIP,
			DstMask: dstMask,
			DstPort: htons(cr.DstPort),
			Proto:   protoToUint8(cr.Proto),
			Action:  actionToUint8(cr.Action),
			RuleID:  cr.ID,
		}

		if err := rulesMap.Put(i, rule); err != nil {
			fmt.Printf("Failed to update rule at index %d: %v\n", i, err)
		} else {
			fmt.Printf("Synced rule %d (Priority %d) to index %d\n", cr.ID, cr.Priority, i)
		}
	}

	// 3. Update the rule count in config map
	if err := configMap.Put(uint32(0), i); err != nil {
		fmt.Printf("Failed to update rule count: %v\n", err)
	} else {
		fmt.Printf("Updated active rule count to %d\n", i)
	}
}

func actionToUint8(action string) uint8 {
	switch strings.ToUpper(action) {
	case "PASS":
		return 1
	case "DROP":
		return 0
	default:
		return 0
	}
}
