# P2 iperf 验收报告

- **时间**: 2026-05-22T03:44:30+00:00
- **网关 WAN**: `157.15.107.249` · **LAN**: `ens19`
- **客户端**: `100.64.0.254` · **网段**: `100.64.0.0/24` · **/32**: `100.64.0.254` @ `20mbit`
- **容差**: ±15% · **时长**: 10s

| 项 | 结果 |
|----|------|
| PASS | 11 |
| FAIL | 0 |
| SKIP | 0 |

```
=== qosnat2 acceptance-p2-iperf 2026-05-22T03:43:42+00:00 ===
OK   API login
OK   profile 100.64.0.0/24 API down_bps=1000000 up_bps=1000000 → expect down≈8.0 up≈8.0 Mbit/s
OK   segment upload (ifb/上行) 8.18 Mbit/s (expect 8.0 ±15%)
OK   segment download -R (ens19 egress) 9.02 Mbit/s (expect 8.0 ±15%)
OK   ens19 egress BPF (下行分类)
OK   wizard 100.64.0.254/32 20mbit
OK   /32 profile down_bps=2500000 → expect ≈20.0 Mbit/s
OK   host_exact map 100.64.0.254 down_bps=2500000
OK   profile_lpm /32 100.64.0.254 down_bps=2500000
OK   /32 upload 19.3 Mbit/s (expect 20.0 ±15%)
OK   /32 download -R 20.6 Mbit/s (expect 20.0 ±15%)
```
