// SPDX-License-Identifier: GPL-2.0
// NAT-QoS 每 IP 令牌桶限速 — TC clsact (ingress=上行, egress=下行)
#include <linux/bpf.h>
#include <linux/if_ether.h>
#include <linux/ip.h>
#include <linux/pkt_cls.h>
#include <bpf/bpf_helpers.h>
#include <bpf/bpf_endian.h>

#define DIR_UP     0
#define DIR_DOWN   1
#define KEY_DEFAULT 0

struct ip_rates {
	__u64 up_bps;
	__u64 up_burst;
	__u64 down_bps;
	__u64 down_burst;
};

struct rate_state {
	__u64 tokens;
	__u64 last_ns;
};

/* 每 IP 精确覆盖；key=IPv4 网络序 */
struct {
	__uint(type, BPF_MAP_TYPE_HASH);
	__uint(max_entries, 65536);
	__type(key, __u32);
	__type(value, struct ip_rates);
} config_map SEC(".maps");

/* LPM trie key: prefixlen + IPv4 */
struct lpm_v4_key {
	__u32 prefixlen;
	__u8 addr[4];
};

/* 网段限速（最长前缀匹配），如 10.100.0.0/16 */
struct {
	__uint(type, BPF_MAP_TYPE_LPM_TRIE);
	__uint(max_entries, 1024);
	__uint(map_flags, BPF_F_NO_PREALLOC);
	__type(key, struct lpm_v4_key);
	__type(value, struct ip_rates);
} cidr_map SEC(".maps");

/* 令牌桶状态；key=(ip<<1)|dir */
struct {
	__uint(type, BPF_MAP_TYPE_LRU_HASH);
	__uint(max_entries, 262144);
	__type(key, __u64);
	__type(value, struct rate_state);
} state_map SEC(".maps");

/* 部署时由 loader 写入 */
const volatile __u64 default_up_bps = 500000;   /* 4 Mbit/s */
const volatile __u64 default_up_burst = 65536;
const volatile __u64 default_down_bps = 1250000; /* 10 Mbit/s */
const volatile __u64 default_down_burst = 163840;

static __always_inline void pick_rate(struct ip_rates *r, int dir, __u64 *rate, __u64 *burst)
{
	if (dir == DIR_UP) {
		*rate = r->up_bps;
		*burst = r->up_burst;
	} else {
		*rate = r->down_bps;
		*burst = r->down_burst;
	}
}

static __always_inline int rates_valid(struct ip_rates *r)
{
	return r && (r->up_bps || r->down_bps);
}

static __always_inline struct ip_rates *lookup_rates(__u32 ip_be)
{
	struct ip_rates *r = bpf_map_lookup_elem(&config_map, &ip_be);
	if (rates_valid(r))
		return r;

	struct lpm_v4_key lpm = {
		.prefixlen = 32,
	};
	__builtin_memcpy(lpm.addr, &ip_be, 4);
	r = bpf_map_lookup_elem(&cidr_map, &lpm);
	if (rates_valid(r))
		return r;

	__u32 def_key = KEY_DEFAULT;
	return bpf_map_lookup_elem(&config_map, &def_key);
}

static __always_inline int do_rate_limit(struct __sk_buff *skb, __u32 ip_be, int dir)
{
	struct ip_rates *rates = lookup_rates(ip_be);
	__u64 rate_bps, burst;
	if (rates) {
		pick_rate(rates, dir, &rate_bps, &burst);
	} else {
		rate_bps = (dir == DIR_UP) ? default_up_bps : default_down_bps;
		burst = (dir == DIR_UP) ? default_up_burst : default_down_burst;
	}
	if (!rate_bps)
		return TC_ACT_OK;

	__u64 key = ((__u64)ip_be << 1) | (__u64)dir;
	struct rate_state *st = bpf_map_lookup_elem(&state_map, &key);
	struct rate_state new_st = {};
	__u64 now = bpf_ktime_get_ns();
	/* skb->len 多为 L3 长度，补偿以太网/IP/TCP 头开销使速率更接近 mbit 标称 */
	__u32 pkt_len = skb->len + 24;

	if (st) {
		new_st = *st;
	} else {
		new_st.tokens = burst;
		new_st.last_ns = now;
	}

	if (new_st.last_ns && rate_bps) {
		__u64 delta = now - new_st.last_ns;
		/* 向上取整补令牌，减少系统性偏低 */
		__u64 add = (delta * rate_bps + 999999999ULL) / 1000000000ULL;
		new_st.tokens += add;
		if (new_st.tokens > burst)
			new_st.tokens = burst;
	}
	new_st.last_ns = now;

	if (new_st.tokens >= pkt_len) {
		new_st.tokens -= pkt_len;
		bpf_map_update_elem(&state_map, &key, &new_st, BPF_ANY);
		return TC_ACT_OK;
	}

	bpf_map_update_elem(&state_map, &key, &new_st, BPF_ANY);
	return TC_ACT_SHOT;
}

static __always_inline int handle_ip(struct __sk_buff *skb, int dir)
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

	__u32 addr = (dir == DIR_UP) ? ip->saddr : ip->daddr;
	return do_rate_limit(skb, addr, dir);
}

SEC("tc/ingress")
int tc_ingress(struct __sk_buff *skb)
{
	return handle_ip(skb, DIR_UP);
}

SEC("tc/egress")
int tc_egress(struct __sk_buff *skb)
{
	return handle_ip(skb, DIR_DOWN);
}

char LICENSE[] SEC("license") = "GPL";
