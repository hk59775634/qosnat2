/* SPDX-License-Identifier: GPL-2.0 */
#ifndef __QOSNAT2_H
#define __QOSNAT2_H

#define QOSNAT_MAJ 1

struct rate_val {
	__u64 down_bps;
	__u64 up_bps;
	__u32 class_minor;
	__u8  pad[4];
};

struct lpm_v4_key {
	__u32 prefixlen;
	__u32 addr;
};

struct active_val {
	__u64 bytes_down;
	__u64 bytes_up;
	__u64 last_seen_ns;
	__u32 class_minor;
	__u32 flags;
};

/* ringbuf → Go 动态建 HTB 类 */
struct new_host_event {
	__u32 ip_be;
	__u64 down_bps;
	__u64 up_bps;
	__u32 class_minor;
	__u32 _pad;
};

#endif
