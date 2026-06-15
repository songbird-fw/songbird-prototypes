package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/cilium/ebpf"
	"github.com/cilium/ebpf/link"
	"github.com/cilium/ebpf/perf"
	"github.com/cilium/ebpf/rlimit"
)

type RuleKey struct {
	SrcIP   uint32
	DstIP   uint32
	DstPort uint16
	Proto   uint8
	_       uint8 // Padding to align with C struct
}

type RuleValue struct {
	Action uint8
	_      [3]uint8 // Padding
	RuleID uint32
}

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

func main() {
	fmt.Println("=== BPF Firewall PROTOTYPE ===")
	fmt.Println("🔍 Advanced Logging & Scalable Rules (DEFAULT DROP)")

	if err := rlimit.RemoveMemlock(); err != nil {
		fmt.Fprintf(os.Stderr, "⚠ %s\n", err)
	}

	spec, err := ebpf.LoadCollectionSpec("bpf/firewall.bpf.o")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Compiling BPF program...\n")
		cmd := exec.Command("clang", "-O2", "-g", "-target", "bpf", "-I/usr/include/x86_64-linux-gnu", "-c", "bpf/firewall.c", "-o", "bpf/firewall.bpf.o")
		output, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Compilation failed: %s\n%s\n", err, string(output))
			os.Exit(1)
		}
		spec, err = ebpf.LoadCollectionSpec("bpf/firewall.bpf.o")
		if err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to load spec: %s\n", err)
			os.Exit(1)
		}
	}

	coll, err := ebpf.NewCollection(spec)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ %s\n", err)
		os.Exit(1)
	}
	defer coll.Close()

	xdpProg := coll.Programs["firewall_filter"]
	eventsMap := coll.Maps["events"]
	rulesMap := coll.Maps["rules"]

	// Populate rules
	populateRules(rulesMap)

	fmt.Println("\n=== Getting IP of this machine ===")
	addrs, _ := net.InterfaceAddrs()
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				fmt.Printf("  • %s\n", ipnet.IP.String())
			}
		}
	}

	fmt.Println("\n=== Interfaces ===")
	interfaces, _ := net.Interfaces()
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp != 0 && iface.Flags&net.FlagLoopback == 0 {
			fmt.Printf("  • %s (idx=%d)\n", iface.Name, iface.Index)
		}
	}

	fmt.Print("\nInterface to attach (or press Enter for 'eth0'): ")
	var ifaceName string
	fmt.Scanln(&ifaceName)
	if ifaceName == "" {
		ifaceName = "eth0"
	}

	iface, err := net.InterfaceByName(ifaceName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Interface not found: %s\n", err)
		os.Exit(1)
	}

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

			var event PacketEvent
			if err := binary.Read(bytes.NewReader(record.RawSample), binary.LittleEndian, &event); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to decode event: %s\n", err)
				continue
			}

			printEvent(event)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	fmt.Println("\nShutting down...")
}

func populateRules(rulesMap *ebpf.Map) {
	rules := []struct {
		key   RuleKey
		value RuleValue
	}{
		// Rule 1: Allow TCP port 22 (SSH) for everyone - SAFETY FIRST
		{
			key: RuleKey{
				DstPort: htons(22),
				Proto:   6, // TCP
			},
			value: RuleValue{Action: 1, RuleID: 22},
		},
		// Rule 2: Allow TCP port 443 (HTTPS) for everyone
		{
			key: RuleKey{
				DstPort: htons(443),
				Proto:   6, // TCP
			},
			value: RuleValue{Action: 1, RuleID: 443},
		},
		// Rule 3: Allow ICMP (Ping)
		{
			key: RuleKey{
				Proto: 1, // ICMP
			},
			value: RuleValue{Action: 1, RuleID: 1},
		},
		// Rule 4: Specifically Block TCP port 80 (HTTP) - already dropped by default, but let's label it
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

func ipToUint32(ipStr string) uint32 {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0
	}
	return binary.LittleEndian.Uint32(ip)
}

func htons(i uint16) uint16 {
	b := make([]byte, 2)
	binary.BigEndian.PutUint16(b, i)
	return binary.LittleEndian.Uint16(b)
}

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

	duration := time.Duration(e.Timestamp) * time.Nanosecond

	fmt.Printf("[%12s] %s | %s | %s:%d -> %s:%d | Rule=%d\n",
		duration.String(), actionStr, protoStr, srcIP, ntohs(e.SrcPort), dstIP, ntohs(e.DstPort), e.RuleID)
}

func ntohs(i uint16) uint16 {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, i)
	return binary.BigEndian.Uint16(b)
}
