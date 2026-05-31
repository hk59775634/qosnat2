# High availability and multi-POP deployment

qosnat2 is a **single-node control plane**. HA is achieved at the architecture layer, not inside one process.

## Single-node resilience

| Component | Mitigation |
|-----------|------------|
| qosnatd crash | `systemctl restart=qosnatd` |
| state corruption | Atomic writes + [OPS-BACKUP.md](./OPS-BACKUP.md) |
| nft/dataplane drift | Re-apply from UI or import state |

## Active / standby (cold)

1. Two identical gateways; **one active** with public IPs and DNS
2. Nightly or continuous `state.json` sync to standby
3. Failover: promote standby, move IPs (BGP, keepalived, or manual), start qosnatd
4. VPN clients reconnect (no built-in session migration)

## Multi-POP / scale-out

For large user counts:

- **DNS or Anycast** steers users to nearest POP
- Each POP runs independent qosnat2 + `state.json` (or templated deploy)
- RADIUS/auth can be central; data plane is per edge
- Avoid per-user nft rules — use aliases/ipset aggregates

## WARP / multi-WAN

`reconcileWarpAfterNft` runs after nft reload; validate WARP paths after failover.

## Out of scope

- Built-in Raft/clustered state
- Cross-node conntrack sync
- Zero-downtime nft reload (full table reload remains)

Use external orchestration (Terraform, Ansible) to keep POP configs consistent.
