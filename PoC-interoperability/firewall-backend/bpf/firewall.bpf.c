#include "vmlinux.h"
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <bpf/bpf_endian.h>
#include <bpf/bpf_helpers.h>

char LICENSE[] SEC("license") = "Dual BSD/GPL";

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 65535);
    __type(key, __u32);
    __type(value, __u8);
} blocked_ips SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 2);
    __type(key, __u32);
    __type(value, __u64);
} stats SEC(".maps");

static __always_inline void incr_stat(__u32 key) {
    __u64 *value = bpf_map_lookup_elem(&stats, &key);
    if (value) {
        __sync_fetch_and_add(value, 1);
    }
}

SEC("xdp")
int xdp_firewall(struct xdp_md *ctx) {
    void *data_end = (void *)(long)ctx->data_end;
    void *data = (void *)(long)ctx->data;
    struct ethhdr *eth = data;
    if ((void *)(eth + 1) > data_end) return XDP_PASS;
    if (eth->h_proto != bpf_htons(ETH_P_IP)) { incr_stat(0); return XDP_PASS; }
    struct iphdr *iph = data + sizeof(*eth);
    if ((void *)(iph + 1) > data_end) return XDP_PASS;
    __u32 src = iph->saddr;
    __u8 *blocked = bpf_map_lookup_elem(&blocked_ips, &src);
    if (blocked) { incr_stat(1); return XDP_DROP; }
    incr_stat(0);
    return XDP_PASS;
}
