#include <linux/bpf.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/tcp.h>
#include <linux/udp.h>
#include <stdbool.h>

#define ETH_P_IP 0x0800
#define IPPROTO_TCP 6
#define IPPROTO_UDP 17
#define IPPROTO_ICMP 1

#define MAX_RULES 256

/*
 * FirewallRule: Represents a single rule in the ordered array.
 */
struct firewall_rule {
    __be32 src_ip;
    __be32 src_mask;
    __be32 dst_ip;
    __be32 dst_mask;
    __be16 dst_port; // 0 = wildcard
    __u8 proto;     // 0 = wildcard
    __u8 action;    // 0: DROP, 1: PASS
    __u32 rule_id;
};

/*
 * PacketEvent: Data structure sent to userspace for every packet.
 */
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

/*
 * Map 'rules': An array storing ordered firewall rules.
 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, MAX_RULES);
    __type(key, __u32);
    __type(value, struct firewall_rule);
} rules SEC(".maps");

/*
 * Map 'config': Stores the number of active rules.
 */
struct {
    __uint(type, BPF_MAP_TYPE_ARRAY);
    __uint(max_entries, 1);
    __type(key, __u32);
    __type(value, __u32);
} config SEC(".maps");

/*
 * Map 'events': A Perf Event Array used to stream packet summaries to userspace.
 */
struct {
    __uint(type, BPF_MAP_TYPE_PERF_EVENT_ARRAY);
    __uint(max_entries, 1024);
} events SEC(".maps");

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

    __u32 *rule_count = bpf_map_lookup_elem(&config, &(__u32){0});
    __u32 num_rules = rule_count ? *rule_count : 0;
    if (num_rules > MAX_RULES) num_rules = MAX_RULES;

    __u8 action = 0; // DEFAULT ACTION: DROP
    __u32 rule_id = 0;
    bool matched = false;

    // Linear search through prioritized rules
    #pragma unroll
    for (__u32 i = 0; i < MAX_RULES; i++) {
        if (i >= num_rules) break;

        struct firewall_rule *rule = bpf_map_lookup_elem(&rules, &i);
        if (!rule) break;

        // CIDR Match for Source IP
        if ((src_ip & rule->src_mask) != (rule->src_ip & rule->src_mask))
            continue;

        // CIDR Match for Destination IP
        if ((dst_ip & rule->dst_mask) != (rule->dst_ip & rule->dst_mask))
            continue;

        // Protocol Match (if not wildcard)
        if (rule->proto != 0 && rule->proto != protocol)
            continue;

        // Destination Port Match (if not wildcard)
        if (rule->dst_port != 0 && rule->dst_port != dst_port)
            continue;

        // Match found!
        action = rule->action;
        rule_id = rule->rule_id;
        matched = true;
        break;
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
