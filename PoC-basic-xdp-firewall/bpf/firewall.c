#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>

#define ETH_P_IP 0x0800
#define IPPROTO_TCP 6
#define IPPROTO_UDP 17
#define IPPROTO_ICMP 1

struct rule_key {
    __be32 src_ip;
    __be32 dst_ip;
    __be16 dst_port;
    __u8 proto;
};

struct rule_value {
    __u8 action; // 0: DROP, 1: PASS
    __u32 rule_id;
};

struct packet_event {
    __be32 src_ip;
    __be32 dst_ip;
    __be16 src_port;
    __be16 dst_port;
    __u8 proto;
    __u8 action;
    __u32 rule_id;
    __u64 timestamp;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, struct rule_key);
    __type(value, struct rule_value);
} rules SEC(".maps");

struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 1024);
} events SEC(".maps");

static __always_inline struct rule_value *lookup_rule(__be32 src_ip, __be32 dst_ip, __be16 dst_port, __u8 proto) {
    struct rule_key key = {};
    struct rule_value *val;

    // We want to support wildcards. A real-world firewall would use a LPM (Longest Prefix Match) trie
    // but for this PoC Hash map, we'll try a few combinations.

    // 1. Exact match (src, dst, port, proto)
    key.src_ip = src_ip;
    key.dst_ip = dst_ip;
    key.dst_port = dst_port;
    key.proto = proto;
    val = bpf_map_lookup_elem(&rules, &key);
    if (val) return val;

    // 2. Wildcard Src IP (any, dst, port, proto)
    key.src_ip = 0;
    val = bpf_map_lookup_elem(&rules, &key);
    if (val) return val;

    // 3. Wildcard Src IP & Dst IP (any, any, port, proto) - common for "port" rules
    key.dst_ip = 0;
    val = bpf_map_lookup_elem(&rules, &key);
    if (val) return val;

    // 4. Wildcard Src IP, Dst IP & Dst Port (any, any, any, proto) - common for "protocol" rules
    key.dst_port = 0;
    val = bpf_map_lookup_elem(&rules, &key);
    if (val) return val;

    return NULL;
}

SEC("xdp")
int firewall_filter(struct xdp_md *ctx) {
    void *data = (void *)(long)ctx->data;
    void *data_end = (void *)(long)ctx->data_end;
    struct ethhdr *eth = data;
    struct iphdr *iph;

    if (data + sizeof(*eth) > data_end)
        return XDP_PASS;

    if (eth->h_proto != bpf_htons(ETH_P_IP))
        return XDP_PASS;

    iph = data + sizeof(*eth);
    if ((void *)iph + sizeof(*iph) > data_end)
        return XDP_PASS;

    __be32 src_ip = iph->saddr;
    __be32 dst_ip = iph->daddr;
    __u8 protocol = iph->protocol;
    __be16 src_port = 0;
    __be16 dst_port = 0;

    if (protocol == IPPROTO_TCP) {
        struct tcphdr *tcph = (void *)iph + sizeof(*iph);
        if ((void *)tcph + sizeof(*tcph) <= data_end) {
            src_port = tcph->source;
            dst_port = tcph->dest;
        }
    } else if (protocol == IPPROTO_UDP) {
        struct udphdr *udph = (void *)iph + sizeof(*iph);
        if ((void *)udph + sizeof(*udph) <= data_end) {
            src_port = udph->source;
            dst_port = udph->dest;
        }
    }

    struct rule_value *rule = lookup_rule(src_ip, dst_ip, dst_port, protocol);
    __u8 action = 1; // Default PASS
    __u32 rule_id = 0;

    if (rule) {
        action = rule->action;
        rule_id = rule->rule_id;
    }

    struct packet_event evt = {
        .src_ip = src_ip,
        .dst_ip = dst_ip,
        .src_port = src_port,
        .dst_port = dst_port,
        .proto = protocol,
        .action = action,
        .rule_id = rule_id,
        .timestamp = bpf_ktime_get_ns()
    };

    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));

    if (action == 0) {
        return XDP_DROP;
    }

    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
