# songbird-prototypes

## 1. Proof of Concept - Basic XDP firewall in Go

This PoC demonstrates a scalable eBPF/XDP firewall. Rules are defined in Go and enforced in the kernel via a BPF Hash Map.

### Prerequisites

To compile and run this PoC, you need:
- **Go 1.18+**
- **Clang/LLVM** (for compiling the eBPF C code)
- **libbpf-dev** and **libelf-dev** (kernel eBPF development headers)
- **Linux kernel 5.4+** with XDP support

On Ubuntu/Debian, you can install the dependencies with:
```bash
sudo apt-get update
sudo apt-get install -y clang llvm libbpf-dev libelf-dev
```

### How to build into a single binary

To ship this firewall as a single binary, you need to compile the eBPF code first and then build the Go application, which embeds the resulting BPF object file.

1. **Compile the eBPF program:**
   ```bash
   cd PoC-basic-xdp-firewall
   clang -O2 -g -target bpf -I/usr/include/x86_64-linux-gnu -c bpf/firewall.c -o bpf/firewall.bpf.o
   ```

2. **Build the Go binary:**
   ```bash
   go build -o firewall main.go
   ```

The resulting `firewall` binary is self-contained and includes the compiled eBPF program.

### Usage

1. **Run the firewall (requires root for eBPF operations):**
   ```bash
   cd PoC-basic-xdp-firewall
   sudo ./firewall
   ```

2. **Select Interface:**
   The program will list available interfaces. Type the name of the interface you want to attach to (e.g., `eth0` or `lo`).

3. **Monitor Traffic:**
   Once attached, the program will log matches and actions (PASS/DROP) in real-time.

### Testing

You can run the included tests to verify that BPF maps and programs are loading correctly:
```bash
cd PoC-basic-xdp-firewall
sudo go test -v .
```

### Features
- **Scalable Rules**: Uses BPF Hash Maps for efficient rule lookup.
- **Dynamic Management**: Rules are managed from userspace (Go) without needing to recompile the kernel-side code.
- **Wildcard Support**: Supports rules for specific IPs, ports, or entire protocols.
- **Real-time Logging**: Uses BPF Perf Events to send packet summaries to userspace for monitoring.
