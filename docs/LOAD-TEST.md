# Load testing handbook

qosnat2 capacity is **data-plane bound** (nft, conntrack, VPN crypto, tc), not Admin API QPS. Use this checklist before declaring production readiness.

## Goals

| Scenario | Metric |
|----------|--------|
| Firewall/NAT change | nft reload latency p99 < target (e.g. 2s @ your rule count) |
| VPN throughput | WireGuard or OpenConnect at expected cipher + PPS |
| Concurrent VPN users | CPU, conntrack, memory stable at target count |
| QoS | Shaper tenant count without tc/BPF errors |

See [PRODUCTION_READINESS.md](../自动代码审计/PRODUCTION_READINESS.md) for rough single-node limits.

## Tools

- **Throughput**: `iperf3` through WAN/LAN paths
- **Many flows**: `hping3`, custom scripts, or commercial traffic generators
- **conntrack**: `conntrack -C`, tune `nf_conntrack_max` via System → Tuning
- **API smoke**: `scripts/acceptance-check.sh` (HTTP only)

## Procedure

1. Snapshot `state.json` ([OPS-BACKUP.md](./OPS-BACKUP.md))
2. Baseline: `curl -s localhost:8080/api/v1/metrics` (Prometheus) during idle
3. Apply representative config (rules, NAT, one VPN profile)
4. Ramp load; watch `dmesg`, `nft list ruleset | wc -l`, `ss -s`, CPU per core
5. Change one firewall rule during load — measure reload gap/loss
6. Record results in change ticket

## Prometheus alerts

Example rules in [PROMETHEUS-METRICS.md](./PROMETHEUS-METRICS.md) (`QosnatNftReloadSlow`, `QosnatNatStackSlow`).

## Multi-node / 100k users

Single qosnat2 instance is not designed for 100k concurrent VPN on one box. Use multiple edge POPs — see [HA-DEPLOYMENT.md](./HA-DEPLOYMENT.md).
