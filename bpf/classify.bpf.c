// SPDX-License-Identifier: GPL-2.0
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include "headers/qosnat2.h"

const volatile int ifb_ifindex;

struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 4096);
	__uint(map_flags, BPF_F_NO_PREALLOC);
	__type(key, struct lpm_v4_key);
	__type(value, struct rate_val);
} profile_lpm SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, __u32);
	__type(value, struct rate_val);
} host_exact SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 131072);
	__type(key, __u32);
	__type(value, struct active_val);
} active_host SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 131072);
	__type(key, __u32);
	__type(value, __u32);
} classid_map SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_RINGBUF);
	__uint(max_entries, 1 << 22);
} events SEC(".maps");

static __always_inline __u32 class_minor_for(__u32 ip_key, struct rate_val *rv)
{
	if (rv && rv->class_minor)
		return rv->class_minor;
	__u32 m = 0x100 | (ip_key & 0xffff);
	if (m == QOSNAT_MAJ)
		m++;
	return m;
}

/*
 * Map 键与 Go 一致：host_exact 用 BigEndian.PutUint32(IPToHostKey(ip))，
 * 即 skb 内 saddr/daddr 四字节（网络序）作为 __u32 键。
 * LPM trie 的 addr 字段与 Go IPToLPMKey 相同，用 bpf_ntohl(addr_be)。
 */
static __always_inline struct rate_val *lookup_rate(__u32 addr_be)
{
	struct rate_val *rv = bpf_map_lookup_elem(&host_exact, &addr_be);
	if (rv)
		return rv;

	struct lpm_v4_key key = {
		.prefixlen = 32,
		.addr = bpf_ntohl(addr_be),
	};
	rv = bpf_map_lookup_elem(&profile_lpm, &key);
	if (rv)
		return rv;

	key.prefixlen = 0;
	key.addr = 0;
	return bpf_map_lookup_elem(&profile_lpm, &key);
}

static __always_inline int handle_l3(struct __sk_buff *skb, __u32 addr_be, int ingress)
{
	if (!addr_be)
		return TC_ACT_OK;

	struct rate_val *rv = lookup_rate(addr_be);
	if (!rv)
		return TC_ACT_OK;

	__u32 ip_host = bpf_ntohl(addr_be);
	__u32 minor = class_minor_for(ip_host, rv);
	/* direct-action：仅低 16 位生效，major 由 tc filter classid 1:0 提供 */
	skb->tc_classid = minor;

	__u32 *cached = bpf_map_lookup_elem(&classid_map, &addr_be);
	if (!cached || *cached != minor)
		bpf_map_update_elem(&classid_map, &addr_be, &minor, BPF_ANY);

	struct active_val *act = bpf_map_lookup_elem(&active_host, &addr_be);
	__u64 now = bpf_ktime_get_ns();
	int is_new = !act;

	struct active_val upd = {};
	if (act)
		upd = *act;
	upd.class_minor = minor;
	upd.last_seen_ns = now;
	if (ingress)
		upd.bytes_up += skb->len;
	else
		upd.bytes_down += skb->len;
		bpf_map_update_elem(&active_host, &addr_be, &upd, BPF_ANY);

	if (is_new) {
		struct new_host_event *ev = bpf_ringbuf_reserve(&events, sizeof(*ev), 0);
		if (ev) {
			ev->ip_be = ip_host;
			ev->down_bps = rv->down_bps;
			ev->up_bps = rv->up_bps;
			ev->class_minor = minor;
			bpf_ringbuf_submit(ev, 0);
		}
	}

	/* 上行整形由 TC mirred 将 ingress 流量导入 ifb（见 internal/ebpf/attach.go） */
	(void)ifb_ifindex;
	return TC_ACT_OK;
}

#define ETH_HLEN 14

static __always_inline int parse_ipv4_at(struct __sk_buff *skb, int ingress, __u32 l3_off)
{
	__u8 vhl;
	if (bpf_skb_load_bytes(skb, l3_off, &vhl, sizeof(vhl)))
		return TC_ACT_OK;
	if ((vhl & 0xf0) != 0x40)
		return TC_ACT_OK;

	__u32 addr_be;
	__u32 aoff = l3_off + (ingress ? 12 : 16);
	if (bpf_skb_load_bytes(skb, aoff, &addr_be, sizeof(addr_be)))
		return TC_ACT_OK;

	return handle_l3(skb, addr_be, ingress);
}

static __always_inline int parse_ipv4(struct __sk_buff *skb, int ingress)
{
	if (bpf_skb_pull_data(skb, 0))
		return TC_ACT_OK;

	__u16 eth_proto;
	if (!bpf_skb_load_bytes(skb, 12, &eth_proto, sizeof(eth_proto)) &&
	    eth_proto == bpf_htons(ETH_P_IP))
		return parse_ipv4_at(skb, ingress, ETH_HLEN);

	/* mirred → ifb0 时 skb 可能从 L3 起，无以太头 */
	return parse_ipv4_at(skb, ingress, 0);
}

SEC("tc/ingress")
int classify_ingress(struct __sk_buff *skb)
{
	return parse_ipv4(skb, 1);
}

SEC("tc/egress")
int classify_egress(struct __sk_buff *skb)
{
	return parse_ipv4(skb, 0);
}

char LICENSE[] SEC("license") = "GPL";
