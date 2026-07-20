// SPDX-License-Identifier: GPL-2.0
/* Per-IP QoS：下行 EDT + fq，上行 token bucket（无 IFB/HTB/ringbuf） */
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>
#include "headers/qosnat2.h"
#include "headers/edt.h"

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
	__type(key, struct throttle_key);
	__type(value, struct throttle_val);
} throttle SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 131072);
	__type(key, struct token_bucket_key);
	__type(value, struct token_bucket_val);
} token_bucket SEC(".maps");

struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 131072);
	__type(key, __u32); /* flow host IP (network order) */
	__type(value, struct host_flow_val);
} host_flow SEC(".maps");

static __always_inline __u32 wire_len(struct __sk_buff *skb)
{
	__u64 len = (__u64)skb->len;
	if (len > 0xffff)
		len = 0xffff;
	return (__u32)len;
}

/* host_mask<32：把 IP 归并到网段地址，供 throttle/token_bucket 共享限速。 */
static __always_inline __u32 aggregate_ip_be(__u32 ip_be, __u8 host_mask)
{
	if (host_mask == 0 || host_mask >= 32)
		return ip_be;
	__u32 host = bpf_ntohl(ip_be);
	__u32 m = 0xffffffffu << (32 - host_mask);
	return bpf_htonl(host & m);
}

static __always_inline void account_host_flow(__u32 flow_ip_be, __u32 bucket_be, __u8 host_mask,
					     __u32 len, __u8 dir)
{
	if (!flow_ip_be || !len)
		return;
	__u64 now = bpf_ktime_get_ns();
	struct host_flow_val *hf = bpf_map_lookup_elem(&host_flow, &flow_ip_be);
	if (!hf) {
		struct host_flow_val init = {
			.bucket_be = bucket_be,
			.host_mask = host_mask,
			.last_ns = now,
		};
		if (dir == DIR_DOWN)
			init.bytes_down = len;
		else
			init.bytes_up = len;
		bpf_map_update_elem(&host_flow, &flow_ip_be, &init, BPF_ANY);
		return;
	}
	hf->bucket_be = bucket_be;
	hf->host_mask = host_mask;
	hf->last_ns = now;
	if (dir == DIR_DOWN)
		hf->bytes_down += len;
	else
		hf->bytes_up += len;
}

static __always_inline struct rate_val *lookup_rate_v4(__u32 addr_be)
{
	struct rate_val *rv = bpf_map_lookup_elem(&host_exact, &addr_be);
	if (rv)
		return rv;
	struct lpm_v4_key key = { .prefixlen = 32, .addr = addr_be };
	return bpf_map_lookup_elem(&profile_lpm, &key);
}

// aggregate_id fq 流标识：对完整 IPv4 做混合，降低碰撞。
static __always_inline __u32 aggregate_id(__u32 ip_be, __u8 dir)
{
	__u32 host = bpf_ntohl(ip_be);
	__u32 mix = (host & 0xffff) ^ (host >> 16);
	mix = (mix * 0x9e3779b1U) ^ ((__u32)dir << 24);
	__u32 id = (mix % 65534U) + 1U;
	return id;
}

static __always_inline int parse_ipv4_addrs(struct __sk_buff *skb, __u32 l3_off,
					    __u32 *saddr_be, __u32 *daddr_be)
{
	__u8 vhl;
	if (bpf_skb_load_bytes(skb, l3_off, &vhl, sizeof(vhl)))
		return -1;
	if ((vhl & 0xf0) != 0x40)
		return -1;
	if (bpf_skb_load_bytes(skb, l3_off + 12, saddr_be, sizeof(*saddr_be)))
		return -1;
	if (bpf_skb_load_bytes(skb, l3_off + 16, daddr_be, sizeof(*daddr_be)))
		return -1;
	return 0;
}

static __always_inline int l3_offset(struct __sk_buff *skb)
{
	__u8 vhl0;
	if (!bpf_skb_load_bytes(skb, 0, &vhl0, sizeof(vhl0)) && (vhl0 & 0xf0) == 0x40)
		return 0;
	__u16 eth_proto;
	if (!bpf_skb_load_bytes(skb, 12, &eth_proto, sizeof(eth_proto)) &&
	    eth_proto == bpf_htons(ETH_P_IP))
		return 14;
	return 0;
}

/* 下行 EDT：flow_ip 用于 fq 流标识；key_ip 用于共享限速桶（可已按 mask 聚合）。 */
static __always_inline int edt_sched(struct __sk_buff *skb, __u32 flow_ip_be, __u32 key_ip_be,
				     __u64 bps, __u8 host_mask)
{
	if (!key_ip_be || !bps)
		return TC_ACT_OK;

	__u64 now = bpf_ktime_get_ns();
	__u32 len = wire_len(skb);
	__u64 delay = (__u64)len * NSEC_PER_SEC / bps;
	struct throttle_key tk = { .ip_be = key_ip_be, .direction = DIR_DOWN };
	struct throttle_val *info = bpf_map_lookup_elem(&throttle, &tk);
	struct throttle_val init = {
		.t_last = now,
		.bps = bps,
		.t_horizon = EDT_HORIZON_NS,
		.bytes = 0,
	};

	if (!info) {
		bpf_map_update_elem(&throttle, &tk, &init, BPF_ANY);
		info = bpf_map_lookup_elem(&throttle, &tk);
		if (!info)
			return TC_ACT_OK;
	} else if (info->bps != bps) {
		info->bps = bps;
		info->t_horizon = EDT_HORIZON_NS;
	}

	__u64 t = skb->tstamp;
	if (t < now)
		t = now;
	__u64 t_next = info->t_last + delay;
	if (t_next <= t) {
		info->t_last = t;
		info->bytes += len;
		account_host_flow(flow_ip_be, key_ip_be, host_mask, len, DIR_DOWN);
		skb->queue_mapping = aggregate_id(flow_ip_be, DIR_DOWN);
		return TC_ACT_OK;
	}
	if (t_next - now >= info->t_horizon)
		return TC_ACT_SHOT;
	info->t_last = t_next;
	info->bytes += len;
	account_host_flow(flow_ip_be, key_ip_be, host_mask, len, DIR_DOWN);
	skb->tstamp = t_next;
	skb->queue_mapping = aggregate_id(flow_ip_be, DIR_DOWN);
	return TC_ACT_OK;
}

/* 上行 token bucket：key_ip 可为聚合网段地址 */
static __always_inline int bucket_check(struct __sk_buff *skb, __u32 flow_ip_be, __u32 key_ip_be,
					__u64 bps, __u8 host_mask)
{
	if (!key_ip_be || !bps)
		return TC_ACT_OK;

	__u64 now = bpf_ktime_get_ns();
	__u64 cost = (__u64)wire_len(skb);
	__u64 burst_bytes = bps * BUCKET_BURST_NS / NSEC_PER_SEC;
	if (burst_bytes < 1500)
		burst_bytes = 1500;

	struct token_bucket_key bk = { .ip_be = key_ip_be };
	struct token_bucket_val *b = bpf_map_lookup_elem(&token_bucket, &bk);
	struct token_bucket_val fresh = {
		.tokens = burst_bytes,
		.last_ns = now,
		.bps = bps,
		.bytes = 0,
	};

	if (!b) {
		if (cost > burst_bytes)
			return TC_ACT_SHOT;
		fresh.tokens = burst_bytes - cost;
		fresh.bytes = cost;
		bpf_map_update_elem(&token_bucket, &bk, &fresh, BPF_ANY);
		account_host_flow(flow_ip_be, key_ip_be, host_mask, (__u32)cost, DIR_UP);
		return TC_ACT_OK;
	}
	if (b->bps != bps)
		b->bps = bps;

	__u64 elapsed = now - b->last_ns;
	if (elapsed > 0) {
		__u64 add = elapsed * bps / NSEC_PER_SEC;
		b->tokens += add;
		if (b->tokens > burst_bytes)
			b->tokens = burst_bytes;
		b->last_ns = now;
	}
	if (b->tokens >= cost) {
		b->tokens -= cost;
		b->bytes += cost;
		account_host_flow(flow_ip_be, key_ip_be, host_mask, (__u32)cost, DIR_UP);
		return TC_ACT_OK;
	}
	return TC_ACT_SHOT;
}

SEC("tc/ingress")
int rate_limit_ingress(struct __sk_buff *skb)
{
	if (bpf_skb_pull_data(skb, 0))
		return TC_ACT_OK;

	__u32 off = l3_offset(skb);
	__u32 saddr, daddr;
	if (parse_ipv4_addrs(skb, off, &saddr, &daddr) < 0)
		return TC_ACT_OK;

	struct rate_val *rv = lookup_rate_v4(saddr);
	if (!rv || !rv->up_bps)
		return TC_ACT_OK;
	__u32 key = aggregate_ip_be(saddr, rv->host_mask);
	return bucket_check(skb, saddr, key, rv->up_bps, rv->host_mask);
}

SEC("tc/egress")
int rate_limit_egress(struct __sk_buff *skb)
{
	if (bpf_skb_pull_data(skb, 0))
		return TC_ACT_OK;

	__u32 off = l3_offset(skb);
	__u32 saddr, daddr;
	if (parse_ipv4_addrs(skb, off, &saddr, &daddr) < 0)
		return TC_ACT_OK;

	struct rate_val *rv = lookup_rate_v4(daddr);
	if (!rv || !rv->down_bps)
		return TC_ACT_OK;
	__u32 key = aggregate_ip_be(daddr, rv->host_mask);
	return edt_sched(skb, daddr, key, rv->down_bps, rv->host_mask);
}

char LICENSE[] SEC("license") = "GPL";
