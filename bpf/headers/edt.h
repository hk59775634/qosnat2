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
};

struct token_bucket_key {
	__u32 ip_be;
};

struct token_bucket_val {
	__u64 tokens;
	__u64 last_ns;
	__u64 bps;
};

#endif
