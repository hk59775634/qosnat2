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

static __always_inline __u32 class_minor_for(__u32 ip_be, struct rate_val *rv)
{
	if (rv && rv->class_minor)
		return rv->class_minor;
	__u32 m = 0x100 | (ip_be & 0xffff);
	if (m == QOSNAT_MAJ)
		m++;
	return m;
}

static __always_inline struct rate_val *lookup_rate(__u32 ip_be)
{
	struct rate_val *rv = bpf_map_lookup_elem(&host_exact, &ip_be);
	if (rv)
		return rv;

	struct lpm_v4_key key = {
		.prefixlen = 32,
		.addr = ip_be,
	};
	rv = bpf_map_lookup_elem(&profile_lpm, &key);
	if (rv)
		return rv;

	key.prefixlen = 0;
	key.addr = 0;
	return bpf_map_lookup_elem(&profile_lpm, &key);
}

static __always_inline int handle_l3(struct __sk_buff *skb, __u32 ip_be, int ingress)
{
	struct rate_val *rv = lookup_rate(ip_be);
	if (!rv)
		return TC_ACT_OK;

	__u32 minor = class_minor_for(ip_be, rv);
	__u32 classid = (QOSNAT_MAJ << 16) | minor;
	skb->tc_classid = classid;

	__u32 *cached = bpf_map_lookup_elem(&classid_map, &ip_be);
	if (!cached || *cached != minor)
		bpf_map_update_elem(&classid_map, &ip_be, &minor, BPF_ANY);

	struct active_val *act = bpf_map_lookup_elem(&active_host, &ip_be);
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
	bpf_map_update_elem(&active_host, &ip_be, &upd, BPF_ANY);

	if (is_new) {
		struct new_host_event *ev = bpf_ringbuf_reserve(&events, sizeof(*ev), 0);
		if (ev) {
			ev->ip_be = ip_be;
			ev->down_bps = rv->down_bps;
			ev->up_bps = rv->up_bps;
			ev->class_minor = minor;
			bpf_ringbuf_submit(ev, 0);
		}
	}

	if (ingress) {
		if (ifb_ifindex <= 0)
			return TC_ACT_OK;
		return bpf_redirect(ifb_ifindex, BPF_F_INGRESS);
	}
	return TC_ACT_OK;
}

static __always_inline int parse_ipv4(struct __sk_buff *skb, int ingress)
{
	void *data = (void *)(long)skb->data;
	void *data_end = (void *)(long)skb->data_end;
	struct ethhdr *eth = data;

	if ((void *)(eth + 1) > data_end)
		return TC_ACT_OK;
	if (eth->h_proto != bpf_htons(ETH_P_IP))
		return TC_ACT_OK;

	struct iphdr *ip = (void *)(eth + 1);
	if ((void *)(ip + 1) > data_end)
		return TC_ACT_OK;

	__u32 addr = ingress ? ip->saddr : ip->daddr;
	return handle_l3(skb, addr, ingress);
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
