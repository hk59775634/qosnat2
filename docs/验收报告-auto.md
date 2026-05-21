# qosnat2 自动验收报告

- **时间**: 2026-05-21T10:26:54+00:00
- **主机**: ubuntu
- **DEV_LAN**: ens19 **DEV_WAN**: ens18

## 汇总

| 结果 | 数量 |
|------|------|
| PASS | 22 |
| FAIL | 0 |
| SKIP | 2 |

## 日志

```
=== qosnat2 acceptance-auto 2026-05-21T10:26:40+00:00 ===
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
OK   TC flower mirred ingress on ens19 → ifb0
OK   no bpf ingress on ens19
OK   TC bpf on ifb0 parent 1: (upload classify)
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
OK   /api/v1/shaper/hosts (200)
OK   /api/v1/system/general (200)
OK   /api/v1/system/audit (200)
OK   /api/v1/firewall/rules (200)
OK   /api/v1/firewall/geoip (200)
OK   /api/v1/network/vlans (200)
OK   /api/v1/network/wan-links (200)
OK   /api/v1/shaper/tc (200)
OK   /api/v1/api-keys (200)
=== NAT write ===
OK   shared-add (200)
OK   forward-add (200)
=== summary: 34 passed, 0 failed ===
OK   test-ui-api.sh
OK   profile_lpm keys (4) match state (4)
OK   VIP host_exact via API (10.0.0.199)
OK   VIP cleanup DELETE
OK   iperf upload ~8.26 Mbit/s (expect ~8)
OK   restart qosnatd profile_lpm stable (4 keys)
OK   health bpf after restart
OK   after restart: flower mirred on ens19
OK   after restart: bpf on ifb0 parent 1:
OK   conntrack usage 93/2097152
SKIP GeoIP rules (none configured)
SKIP multi-WAN failover (no wan_links)
```
