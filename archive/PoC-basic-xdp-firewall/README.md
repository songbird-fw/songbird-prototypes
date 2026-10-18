# PoC — Basic XDP firewall in Go

| | |
|---|---|
| Question | Can packets be filtered in XDP with rules managed from a Go program? |
| Status | **Superseded** by the Data Plane design in `songbird-architecture/dataplane.md` v3 (archived 2026-10-18) |
| Result | Yes. Its limits (linear scan of 256 rules, no conntrack, IPv4 only, no VLAN, per-packet perf events, non-atomic updates) are listed in `dataplane.md` §2 and drove the new design. `main_test.go` does not compile (`RuleKey`/`RuleValue` do not exist). |
| Decisions | RA C-001 (eBPF Data Plane), ADR-0002, ADR-0003. |

It is still the payload built into the Hadron image by [`PoC-hadron-os`](../../PoC-hadron-os), because it is the only eBPF program available.

## Build

```bash
clang -O2 -g -target bpf -I/usr/include/x86_64-linux-gnu -Ibpf/include -c bpf/firewall.c -o bpf/firewall.bpf.o
CGO_ENABLED=0 go build -o firewall .   # the BPF object is embedded with go:embed
```

Requires clang/LLVM and the kernel headers (`linux-libc-dev` on Debian/Ubuntu). Binaries are not committed.

## Run

```bash
sudo ./firewall   # lists the interfaces, then attaches to the one you type; logs PASS/DROP
```
