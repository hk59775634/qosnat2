# 24h 长稳验收（人工）

建议在业务低峰执行，观察：

1. **NAT**：`watch -n 60 'nft list chain inet qosnat postrouting | head'`
2. **QoS**：`bpftool map dump pinned /sys/fs/bpf/qosnat2/active_host | wc -l` 是否持续增长
3. **conntrack**：`cat /proc/sys/net/netfilter/nf_conntrack_count`
4. **iperf**：每 4h 从客户端跑一次 `acceptance-p2-iperf.sh` 子集

通过标准：无 OOM、`qosnatd` 不重启、活跃池条目在空闲回收后回落。
