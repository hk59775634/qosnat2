# qosnat2 自动验收报告

- **时间**: 2026-05-21T16:10:55+00:00
- **主机**: ubuntu
- **DEV_LAN**: ens19 **DEV_WAN**: ens18

## 汇总

| 结果 | 数量 |
|------|------|
| PASS | 26 |
| FAIL | 1 |
| SKIP | 2 |

## 日志

```
OK   nft SNAT/masquerade
OK   nft asymmetric return drop
OK   API login
OK   API health json
OK   ebpf loaded via API
=== setup ===
OK   setup-status (200)
=== login ===
FAIL login (got 401 want 200) {"error":"invalid credentials"}
提示: 设置 ADMIN_PASS / QOSNAT_PASS，或创建 API Key 后 export QOSNAT_API_KEY=...
FAIL test-ui-api.sh
OK   profile_lpm keys (2) match state (2)
OK   VIP host_exact via API (10.0.0.199)
OK   VIP cleanup DELETE
FAIL iperf upload 9098 Mbit/s (expect 6-11)
OK   iperf download -R ~7.60 Mbit/s (expect ~8)
OK   LAN ingress BPF prio 1 (classify before mirred)
OK   LAN ingress u32 mirred -> ifb0
OK   ifb0 upload u32 filters
OK   restart qosnatd profile_lpm stable (2 keys)
OK   health bpf after restart
OK   after restart: u32 mirred on ens19
FAIL after restart: ifb0 bpf missing
OK   conntrack usage 63/2097152
SKIP GeoIP rules (none configured)
SKIP multi-WAN failover (no wan_links)

=== summary: PASS=23 FAIL=3 SKIP=2 ===
report: /opt/qosnat2/docs/验收报告-auto.md
=== qosnat2 acceptance-auto 2026-05-21T16:10:32+00:00 ===
=== qosnat2 acceptance-check ===
OK   qosnatd active
OK   health HTTP
OK   setup_complete in state
OK   sysctl.d present
OK   nft table qosnat
OK   ifb0 exists
OK   bpf object present
OK   pinned profile_lpm (/sys/fs/bpf/qosnat2)

=== 需人工验收（§14）===
  - 内网单 IP iperf 对比 QoS 模板速率 ±5%
  - VIP /32 覆盖默认网段速率
  - 重启 qosnatd 后 bpftool map 与 state.json 一致

summary: 8 passed, 0 failed
OK   acceptance-check.sh
OK   pinned profile_lpm
OK   TC mirred ingress on ens19 → ifb0
OK   TC u32 mirred prio 10 on ens19
OK   no bpf on ens19 ingress (mirred only)
OK   TC bpf ingress on ifb0 (after mirred)
OK   TC bpf on ens19 parent 1: (download classify)
OK   ifb0 HTB qdisc
OK   nft SNAT/masquerade
OK   nft asymmetric return drop
OK   API login
OK   API health json
OK   ebpf loaded via API
=== setup ===
OK   setup-status (200)
=== login ===
FAIL login (got 401 want 200) {"error":"invalid credentials"}
提示: 设置 ADMIN_PASS / QOSNAT_PASS，或创建 API Key 后 export QOSNAT_API_KEY=...
FAIL test-ui-api.sh
OK   profile_lpm keys (2) match state (2)
OK   VIP host_exact via API (10.0.0.199)
OK   VIP cleanup DELETE
OK   iperf upload ~7.73 Mbit/s (expect ~8)
OK   iperf download -R ~7.60 Mbit/s (expect ~8)
OK   LAN ingress has no BPF (mirred not blocked)
OK   LAN ingress u32 mirred -> ifb0
OK   ifb0 upload u32 filters
OK   restart qosnatd profile_lpm stable (2 keys)
OK   health bpf after restart
OK   after restart: u32 mirred on ens19
OK   after restart: bpf on ifb0 ingress
OK   conntrack usage 71/2097152
SKIP GeoIP rules (none configured)
SKIP multi-WAN failover (no wan_links)
```
