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
)

type ConfigRule struct {
	ID      uint32 `yaml:"id"`
	SrcIP   string `yaml:"src_ip"`
	DstIP   string `yaml:"dst_ip"`
	DstPort uint16 `yaml:"dst_port"`
	Proto   string `yaml:"proto"`
	Action  string `yaml:"action"`
}

type Config struct {
	Rules []ConfigRule `yaml:"rules"`
}

// RuleKey must match the C struct exactly, including padding and alignment.
type RuleKey struct {
	SrcIP   uint32
	DstIP   uint32
	DstPort uint16
	Proto   uint8
	_       uint8 // Padding to align with C struct
}

// RuleValue must match the C struct exactly.
type RuleValue struct {
	Action uint8
	_      [3]uint8 // Padding
	RuleID uint32
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
					syncRules(rulesMap, config)
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
	switch proto {
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

func syncRules(rulesMap *ebpf.Map, config *Config) {
	// 1. Convert config rules to a map for easy lookup
	newRules := make(map[RuleKey]RuleValue)
	for _, cr := range config.Rules {
		key := RuleKey{
			SrcIP:   ipToUint32(cr.SrcIP),
			DstIP:   ipToUint32(cr.DstIP),
			DstPort: htons(cr.DstPort),
			Proto:   protoToUint8(cr.Proto),
		}
		val := RuleValue{
			Action: actionToUint8(cr.Action),
			RuleID: cr.ID,
		}
		newRules[key] = val
	}

	// 2. Iterate through existing BPF map and delete rules not in the new config
	var key RuleKey
	var val RuleValue
	iter := rulesMap.Iterate()
	toDelete := []RuleKey{}
	for iter.Next(&key, &val) {
		if _, ok := newRules[key]; !ok {
			toDelete = append(toDelete, key)
		}
	}
	if err := iter.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Error iterating rules map: %v\n", err)
	}

	for _, k := range toDelete {
		if err := rulesMap.Delete(k); err != nil {
			fmt.Printf("Failed to delete old rule: %v\n", err)
		} else {
			fmt.Println("Deleted old rule from BPF map")
		}
	}

	// 3. Add or update rules from the config
	for k, v := range newRules {
		if err := rulesMap.Put(k, v); err != nil {
			fmt.Printf("Failed to insert/update rule %d: %v\n", v.RuleID, err)
		} else {
			// We could be more quiet here, or only print if it's a new rule or changed
			// For now, let's just log
			fmt.Printf("Synced rule %d\n", v.RuleID)
		}
	}
}

func actionToUint8(action string) uint8 {
	switch action {
	case "PASS":
		return 1
	case "DROP":
		return 0
	default:
		return 0
	}
}
