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
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 4096);
	__uint(map_flags, BPF_F_NO_PREALLOC);
	__type(key, struct lpm_v6_key);
	__type(value, struct rate_val);
} profile_lpm6 SEC(".maps");

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

/* skb 读出的 saddr 在 LE 上需 ntohl 才与 Go IPToHostKey 一致 */
static __always_inline __u32 ip_host_key(__u32 addr_be)
{
	return bpf_ntohl(addr_be);
}

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
 * Map 键与 Go 一致：host_exact / profile_lpm 的 addr 均为 IPv4 四字节
 * 按大端组成的 __u32（与 skb saddr 网络序一致），勿 bpf_ntohl。
 */
static __always_inline struct rate_val *lookup_rate(__u32 addr_be)
{
	struct rate_val *rv = bpf_map_lookup_elem(&host_exact, &addr_be);
	if (rv)
		return rv;

	struct lpm_v4_key key = {
		.prefixlen = 32,
		.addr = addr_be,
	};
	return bpf_map_lookup_elem(&profile_lpm, &key);
}

/* 同时看源/目的：ingress 优先 saddr（与历史一致），再 daddr；egress 优先 daddr（下行），再 saddr（WG 客户端上行）。 */
static __always_inline struct rate_val *lookup_rate_dual(__u32 saddr_be, __u32 daddr_be, int ingress,
							 __u32 *matched_be)
{
	struct rate_val *rv;

	if (ingress) {
		rv = bpf_map_lookup_elem(&host_exact, &saddr_be);
		if (rv) {
			*matched_be = saddr_be;
			return rv;
		}
		rv = bpf_map_lookup_elem(&host_exact, &daddr_be);
		if (rv) {
			*matched_be = daddr_be;
			return rv;
		}
		struct lpm_v4_key ks = { .prefixlen = 32, .addr = saddr_be };
		rv = bpf_map_lookup_elem(&profile_lpm, &ks);
		if (rv) {
			*matched_be = saddr_be;
			return rv;
		}
		struct lpm_v4_key kd = { .prefixlen = 32, .addr = daddr_be };
		rv = bpf_map_lookup_elem(&profile_lpm, &kd);
		if (rv) {
			*matched_be = daddr_be;
			return rv;
		}
		return NULL;
	}

	rv = bpf_map_lookup_elem(&host_exact, &daddr_be);
	if (rv) {
		*matched_be = daddr_be;
		return rv;
	}
	rv = bpf_map_lookup_elem(&host_exact, &saddr_be);
	if (rv) {
		*matched_be = saddr_be;
		return rv;
	}
	struct lpm_v4_key kd = { .prefixlen = 32, .addr = daddr_be };
	rv = bpf_map_lookup_elem(&profile_lpm, &kd);
	if (rv) {
		*matched_be = daddr_be;
		return rv;
	}
	struct lpm_v4_key ks = { .prefixlen = 32, .addr = saddr_be };
	rv = bpf_map_lookup_elem(&profile_lpm, &ks);
	if (rv) {
		*matched_be = saddr_be;
		return rv;
	}
	return NULL;
}

static __always_inline int apply_rate_classify(struct __sk_buff *skb, __u32 addr_be, struct rate_val *rv,
					       int ingress)
{
	if (!addr_be || !rv)
		return TC_ACT_OK;

	__u32 ip_key = ip_host_key(addr_be);
	__u32 minor = class_minor_for(ip_key, rv);
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
			ev->ip_be = ip_key;
			ev->down_bps = rv->down_bps;
			ev->up_bps = rv->up_bps;
			ev->class_minor = minor;
			bpf_ringbuf_submit(ev, 0);
		}
	}

	/* 上行 redirect 由 LAN ingress u32+mirred 完成（见 internal/ebpf/ifb_mirred.go） */
	(void)ifb_ifindex;
	return TC_ACT_OK;
}

static __always_inline int handle_l3(struct __sk_buff *skb, __u32 addr_be, int ingress)
{
	if (!addr_be)
		return TC_ACT_OK;

	struct rate_val *rv = lookup_rate(addr_be);
	if (!rv)
		return TC_ACT_OK;
	return apply_rate_classify(skb, addr_be, rv, ingress);
}

#define ETH_HLEN 14

static __always_inline int parse_ipv4_at(struct __sk_buff *skb, int ingress, __u32 l3_off)
{
	__u8 vhl;
	if (bpf_skb_load_bytes(skb, l3_off, &vhl, sizeof(vhl)))
		return TC_ACT_OK;
	if ((vhl & 0xf0) != 0x40)
		return TC_ACT_OK;

	__u32 saddr_be, daddr_be;
	if (bpf_skb_load_bytes(skb, l3_off + 12, &saddr_be, sizeof(saddr_be)))
		return TC_ACT_OK;
	if (bpf_skb_load_bytes(skb, l3_off + 16, &daddr_be, sizeof(daddr_be)))
		return TC_ACT_OK;

	__u32 matched_be = 0;
	struct rate_val *rv = lookup_rate_dual(saddr_be, daddr_be, ingress, &matched_be);
	if (!rv)
		return TC_ACT_OK;
	return apply_rate_classify(skb, matched_be, rv, ingress);
}

static __always_inline struct rate_val *lookup_rate_v6_dual(__u8 saddr[16], __u8 daddr[16], int ingress)
{
	struct lpm_v6_key ks = { .prefixlen = 128 };
	struct lpm_v6_key kd = { .prefixlen = 128 };
	__builtin_memcpy(ks.addr, saddr, 16);
	__builtin_memcpy(kd.addr, daddr, 16);
	struct rate_val *rv;

	if (ingress) {
		rv = bpf_map_lookup_elem(&profile_lpm6, &ks);
		if (rv)
			return rv;
		return bpf_map_lookup_elem(&profile_lpm6, &kd);
	}
	rv = bpf_map_lookup_elem(&profile_lpm6, &kd);
	if (rv)
		return rv;
	return bpf_map_lookup_elem(&profile_lpm6, &ks);
}

static __always_inline int apply_profile_classify_v6(struct __sk_buff *skb, struct rate_val *rv)
{
	if (!rv)
		return TC_ACT_OK;
	__u32 minor = rv->class_minor;
	if (!minor)
		minor = 0x100;
	skb->tc_classid = minor;
	return TC_ACT_OK;
}

static __always_inline int parse_ipv6_at(struct __sk_buff *skb, int ingress, __u32 l3_off)
{
	__u8 ver;
	if (bpf_skb_load_bytes(skb, l3_off, &ver, sizeof(ver)))
		return TC_ACT_OK;
	if ((ver & 0xf0) != 0x60)
		return TC_ACT_OK;

	__u8 saddr[16], daddr[16];
	if (bpf_skb_load_bytes(skb, l3_off + 8, saddr, 16))
		return TC_ACT_OK;
	if (bpf_skb_load_bytes(skb, l3_off + 24, daddr, 16))
		return TC_ACT_OK;

	struct rate_val *rv = lookup_rate_v6_dual(saddr, daddr, ingress);
	if (!rv)
		return TC_ACT_OK;
	return apply_profile_classify_v6(skb, rv);
}

static __always_inline int parse_ipv6(struct __sk_buff *skb, int ingress)
{
	if (bpf_skb_pull_data(skb, 0))
		return TC_ACT_OK;

	__u8 ver0;
	if (!bpf_skb_load_bytes(skb, 0, &ver0, sizeof(ver0)) && (ver0 & 0xf0) == 0x60)
		return parse_ipv6_at(skb, ingress, 0);

	__u16 eth_proto;
	if (!bpf_skb_load_bytes(skb, 12, &eth_proto, sizeof(eth_proto)) &&
	    eth_proto == bpf_htons(ETH_P_IPV6))
		return parse_ipv6_at(skb, ingress, ETH_HLEN);

	return TC_ACT_OK;
}

static __always_inline int parse_ipv4(struct __sk_buff *skb, int ingress)
{
	if (bpf_skb_pull_data(skb, 0))
		return TC_ACT_OK;

	/* WireGuard/tun 等 RAW 接口：skb 从 IPv4 头起，偏移 12 处不是 ethertype；若误判为以太帧会把 l3_off 置 14 导致无法命中 profile。 */
	__u8 vhl0;
	if (!bpf_skb_load_bytes(skb, 0, &vhl0, sizeof(vhl0)) && (vhl0 & 0xf0) == 0x40)
		return parse_ipv4_at(skb, ingress, 0);
	if ((vhl0 & 0xf0) == 0x60)
		return parse_ipv6_at(skb, ingress, 0);

	__u16 eth_proto;
	if (!bpf_skb_load_bytes(skb, 12, &eth_proto, sizeof(eth_proto))) {
		if (eth_proto == bpf_htons(ETH_P_IP))
			return parse_ipv4_at(skb, ingress, ETH_HLEN);
		if (eth_proto == bpf_htons(ETH_P_IPV6))
			return parse_ipv6_at(skb, ingress, ETH_HLEN);
	}

	/* mirred → ifb0 等 L3 起始但首字节非预期 IPv4 版本时兜底 */
	return parse_ipv4_at(skb, ingress, 0);
}

SEC("tc/ingress")
int classify_ingress(struct __sk_buff *skb)
{
	int r = parse_ipv4(skb, 1);
	parse_ipv6(skb, 1);
	return r;
}

SEC("tc/egress")
int classify_egress(struct __sk_buff *skb)
{
	int r = parse_ipv4(skb, 0);
	parse_ipv6(skb, 0);
	return r;
}

char LICENSE[] SEC("license") = "GPL";
