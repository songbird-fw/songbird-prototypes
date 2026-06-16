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
)

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

	// Inject rules into the BPF 'rules' map
	populateRules(rulesMap)

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

// populateRules: Centralized rule definitions.
func populateRules(rulesMap *ebpf.Map) {
	rules := []struct {
		key   RuleKey
		value RuleValue
	}{
		// SAFETY RULE: Allow SSH (TCP port 22) to prevent locking out the user
		{
			key: RuleKey{
				DstPort: htons(22),
				Proto:   6, // TCP
			},
			value: RuleValue{Action: 1, RuleID: 22},
		},
		// Allow HTTPS
		{
			key: RuleKey{
				DstPort: htons(443),
				Proto:   6, // TCP
			},
			value: RuleValue{Action: 1, RuleID: 443},
		},
		// Allow ICMP (Ping)
		{
			key: RuleKey{
				Proto: 1, // ICMP
			},
			value: RuleValue{Action: 1, RuleID: 1},
		},
		// Explicit DROP for HTTP (Rule ID 80)
		{
			key: RuleKey{
				DstPort: htons(80),
				Proto:   6, // TCP
			},
			value: RuleValue{Action: 0, RuleID: 80},
		},
	}

	for _, r := range rules {
		if err := rulesMap.Put(r.key, r.value); err != nil {
			fmt.Printf("Failed to insert rule %d: %v\n", r.value.RuleID, err)
		} else {
			fmt.Printf("Inserted rule %d\n", r.value.RuleID)
		}
	}
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
