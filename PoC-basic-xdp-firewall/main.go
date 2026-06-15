package main

import (
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

func main() {
        fmt.Println("=== BPF Firewall DEBUG ===")
        fmt.Println("🔍 Logging ALL packets to see what arrives...")

        if err := rlimit.RemoveMemlock(); err != nil {
                fmt.Fprintf(os.Stderr, "⚠ %s\n", err)
        }

        spec, err := ebpf.LoadCollectionSpec("bpf/firewall.bpf.o")
        if err != nil {
                fmt.Fprintf(os.Stderr, "❌ %s\n", err)
                cmd := exec.Command("clang", "-O2", "-g", "-target", "bpf", "-c", "bpf/firewall.c", "-o", "bpf/firewall.bpf.o")
                output, _ := cmd.CombinedOutput()
                fmt.Println(string(output))
                spec, err = ebpf.LoadCollectionSpec("bpf/firewall.bpf.o")
                if err != nil {
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

        fmt.Print("\nInterface to attach: ")
        var ifaceName string
        fmt.Scanln(&ifaceName)

        iface, _ := net.InterfaceByName(ifaceName)
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

        packetCount := 0
        icmpCount := 0
        tcp80Count := 0
        droppedCount := 0
        acceptedCount := 0

        go func() {
                perfReader, _ := perf.NewReader(eventsMap, 1024)
                for {
                        record, err := perfReader.Read()
                        if err != nil {
                                return
                        }
                        if record.RawSample != nil && len(record.RawSample) >= 24 {
                                packetCount++
                                srcIP := uint32(record.RawSample[0])<<24 | uint32(record.RawSample[1])<<16 | uint32(record.RawSample[2])<<8 | uint32(record.RawSample[3])
                                dstIP := uint32(record.RawSample[4])<<24 | uint32(record.RawSample[5])<<16 | uint32(record.RawSample[6])<<8 | uint32(record.RawSample[7])
                                dstPort := uint16(record.RawSample[10])<<8 | uint16(record.RawSample[11])
                                proto := record.RawSample[12]
                                action := record.RawSample[13]

                                srcIPStr := fmt.Sprintf("%d.%d.%d.%d", (srcIP>>24)&0xFF, (srcIP>>16)&0xFF, (srcIP>>8)&0xFF, srcIP&0xFF)
                                dstIPStr := fmt.Sprintf("%d.%d.%d.%d", (dstIP>>24)&0xFF, (dstIP>>16)&0xFF, (dstIP>>8)&0xFF, dstIP&0xFF)

                                if action == 0 {
                                        droppedCount++
                                } else {
                                        acceptedCount++
                                }

                                if proto == 1 {
                                        icmpCount++
                                        fmt.Printf("[%s] 🔴🔴🔴 ICMP #%d: %s -> %s (BLOCKED? %v)\n",
                                                time.Now().Format("15:04:05"), icmpCount, srcIPStr, dstIPStr, action == 0)
                                } else if proto == 6 && dstPort == 80 {
                                        tcp80Count++
                                        fmt.Printf("[%s] 🔴 TCP port 80 #%d: %s -> %s:80 (BLOCKED? %v)\n",
                                                time.Now().Format("15:04:05"), tcp80Count, srcIPStr, dstIPStr, action == 0)
                                } else {
                                        protoStr := "TCP"
                                        if proto == 17 {
                                                protoStr = "UDP"
                                        } else if proto == 1 {
                                                protoStr = "ICMP"
                                        }
                                        fmt.Printf("[%s] 📦 %s: %s -> %s:%d\n",
                                                time.Now().Format("15:04:05"), protoStr, srcIPStr, dstIPStr, dstPort)
                                }
                        }
                }
        }()

        fmt.Println("\n=== DEBUG MODE: Logging ALL packets ===")
        fmt.Println("🔵 Total packets:", packetCount)
        fmt.Println("🔴 ICMP packets:", icmpCount)
        fmt.Println("📕 TCP/80 packets:", tcp80Count)
        fmt.Println("🔴 Dropped:", droppedCount)
        fmt.Println("✅ Accepted:", acceptedCount)
        fmt.Println()
        fmt.Println("Test NOW from 192.168.1.1:")
        fmt.Println("  ping 192.168.1.100")
        fmt.Println()
        fmt.Println("Watch for: 🔴🔴🔴 ICMP ... (BLOCKED? true)")

        sig := make(chan os.Signal, 1)
        signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
        <-sig
}
