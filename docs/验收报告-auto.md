# qosnat2 自动验收报告

- **时间**: 2026-05-22T04:24:20+00:00
- **主机**: test
- **DEV_LAN**: ens19 **DEV_WAN**: ens18

## 汇总

| 结果 | 数量 |
|------|------|
| PASS | 26 |
| FAIL | 1 |
| SKIP | 1 |

## 日志

```
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
OK   login (200)
=== read APIs (UI pages) ===
OK   /api/v1/health (200)
OK   /api/v1/session (200)
OK   /api/v1/stats/dashboard (200)
OK   /api/v1/nat/policy-routes (200)
OK   /api/v1/nat/shared-ips (200)
OK   /api/v1/nat/static-mappings (200)
OK   /api/v1/nat/prefix-mappings (200)
OK   /api/v1/nat/wan-forwards (200)
OK   /api/v1/shaper/profiles (200)
OK   /api/v1/shaper/active (200)
OK   /api/v1/routes (200)
OK   /api/v1/dhcp (200)
OK   /api/v1/vpn/wireguard (200)
OK   /api/v1/diagnostics/captures (200)
OK   /api/v1/diagnostics/conntrack?limit=5 (200)
OK   /api/v1/ebpf/maps (200)
OK   /api/v1/ebpf/programs (200)
OK   /api/v1/system/mark-policy (200)
OK   /api/v1/system/tuning (200)
OK   /api/v1/interfaces/queues (200)
OK   /api/v1/interfaces (200)
OK   /api/v1/system/general (200)
OK   /api/v1/system/audit (200)
OK   /api/v1/firewall/rules (200)
OK   /api/v1/network/vlans (200)
OK   /api/v1/network/wan-links (200)
OK   /api/v1/shaper/tc (200)
OK   /api/v1/api-keys (200)
=== NAT write ===
OK   shared-add (200)
OK   forward-add (200)
=== summary: 32 passed, 0 failed ===
OK   test-ui-api.sh
FAIL profile_lpm keys got=2 want=3
OK   /32 profile via wizard (10.0.0.199)
OK   /32 profile cleanup DELETE
OK   iperf upload ~8.13 Mbit/s (expect ~8)
OK   iperf download -R ~7.60 Mbit/s (expect ~8)
OK   LAN ingress has no BPF (mirred not blocked)
OK   LAN ingress u32 mirred -> ifb0
OK   ifb0 upload u32 filters
OK   restart qosnatd profile_lpm stable (2 keys)
OK   health bpf after restart
OK   after restart: u32 mirred on ens19
OK   after restart: bpf on ifb0 ingress
OK   conntrack usage 116/2097152
SKIP multi-WAN failover (no wan_links)
```
