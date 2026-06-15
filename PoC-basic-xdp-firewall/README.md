# PoC Basic XDP Firewall

This is a proof-of-concept XDP firewall with a Go control plane and a C eBPF data plane.

## How to build into a single binary

To ship this firewall as a single binary, you need to compile the eBPF code first and then build the Go application, which embeds the resulting BPF object file.

### Prerequisites

You need `clang`, `libbpf-dev`, `libelf-dev`, and `linux-headers-generic` installed on your system to compile the eBPF code.

On Ubuntu/Debian:
```bash
sudo apt-get update
sudo apt-get install -y clang libbpf-dev libelf-dev linux-headers-generic
```

### Build Steps

1. **Compile the eBPF program:**
   ```bash
   clang -O2 -g -target bpf -I/usr/include/x86_64-linux-gnu -c bpf/firewall.c -o bpf/firewall.bpf.o
   ```

2. **Build the Go binary:**
   ```bash
   go build -o firewall main.go
   ```

The resulting `firewall` binary is now self-contained and includes the compiled eBPF program. You can ship this binary to other Linux systems. Note that the target system must have a kernel that supports XDP.

## Usage

Run the binary with root privileges:
```bash
sudo ./firewall
```
Follow the prompts to select the network interface to attach the firewall to.
