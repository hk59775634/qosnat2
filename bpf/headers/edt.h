/* SPDX-License-Identifier: GPL-2.0 */
#ifndef __QOSNAT2_EDT_H
#define __QOSNAT2_EDT_H

#include "qosnat2.h"

#define NSEC_PER_SEC 1000000000ULL
/* fq 默认 drop horizon（与内核 sch_fq 一致量级） */
#define EDT_HORIZON_NS (5ULL * NSEC_PER_SEC)
/* 上行令牌桶突发：约 100ms 流量 */
#define BUCKET_BURST_NS (100ULL * 1000000ULL)

#define DIR_DOWN 0
#define DIR_UP   1

struct throttle_key {
	__u32 ip_be;
	__u8  direction;
	__u8  pad[3];
};

struct throttle_val {
	__u64 t_last;
	__u64 bps;
	__u64 t_horizon;
	__u64 bytes; /* 观测：经调度放行的累计字节 */
};

struct token_bucket_key {
	__u32 ip_be;
};

struct token_bucket_val {
	__u64 tokens;
	__u64 last_ns;
	__u64 bps;
	__u64 bytes; /* 观测：经令牌桶放行的累计字节 */
};

/* 每主机流量（共享桶时仍按原始 IP），供观测面展开成员 */
struct host_flow_val {
	__u32 bucket_be;
	__u8  host_mask;
	__u8  pad[3];
	__u64 bytes_down;
	__u64 bytes_up;
	__u64 last_ns;
};

#endif
