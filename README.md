# songbird-prototypes

### 1. Proof of Concept - Basic XDP firewall in GO
Usage:
```
cd PoC-basix-xdp-firewall
clang -O2 -g -target bpf -c bpf/firewall.c -o bpf/firewall.bpf.o
go run main.go
```
