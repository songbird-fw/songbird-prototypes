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

struct firewall_rule {
    __be32 ip;
    __be16 port;
    __u8 proto;
    __u8 action;
    __u32 id;
};

struct packet_event {
    __be32 src_ip;
    __be32 dst_ip;
    __be16 src_port;
    __be16 dst_port;
    __u8 proto;
    __u8 action;
    __u64 timestamp;
};

struct {
    __uint(type, BPF_MAP_TYPE_HASH);
    __uint(max_entries, 1024);
    __type(key, __u32);
    __type(value, struct firewall_rule);
} rules SEC(".maps");

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

    __be32 dst_ip = iph->daddr;
    __be32 src_ip = iph->saddr;
    __u8 protocol = iph->protocol;

    struct packet_event evt = {
        .src_ip = src_ip,
        .dst_ip = dst_ip,
        .src_port = 0,
        .dst_port = 0,
        .proto = protocol,
        .action = 1, // default: ACCEPT
        .timestamp = bpf_ktime_get_ns()
    };

    // ICMP: Block ping from 192.168.1.1 to 192.168.1.100
    if (protocol == IPPROTO_ICMP) {
        if (src_ip == bpf_htonl(0xc0a80101) && dst_ip == bpf_htonl(0xc0a80164)) {
            evt.action = 0; // DROP
            bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
            return XDP_DROP;
        }
        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
        return XDP_PASS;
    }

    // TCP: DROP port 80, PASS port 443
    if (protocol == IPPROTO_TCP) {
        struct tcphdr *tcph = (void *)iph + sizeof(*iph);
        if ((void *)tcph + sizeof(*tcph) > data_end)
            return XDP_PASS;

        __be16 dst_port = tcph->dest;
        evt.src_port = tcph->source;
        evt.dst_port = dst_port;

        if (dst_port == bpf_htons(80)) {
            evt.action = 0; // DROP
            bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
            return XDP_DROP;
        }

        if (dst_port == bpf_htons(443)) {
            evt.action = 1; // PASS
            bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
            return XDP_PASS;
        }

        bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
        return XDP_PASS;
    }

    // All other protocols: log and pass
    bpf_perf_event_output(ctx, &events, BPF_F_CURRENT_CPU, &evt, sizeof(evt));
    return XDP_PASS;
}

char _license[] SEC("license") = "GPL";
