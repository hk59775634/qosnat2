# WireGuard scaling notes

WireGuard data plane runs in the Linux kernel; qosnat2 manages instances, keys, shaper hooks, and nft auto-rules.

## Per-instance limits

- Each WG interface adds listen ports and nft auto-input rules
- Prefer **fewer instances, more peers per instance** when possible
- User traffic stats use maps — very large peer counts need RAM and CPU for periodic collection

## nft interaction

Firewall sync adds auto accept rules for configured WG UDP ports. Bulk instance changes trigger full nft reload — batch UI edits where possible.

## Shaper

Per-tenant WG shaping uses tc + BPF profiles. Thousands of **tenants** is supported in design; thousands of **per-user classes** is not.

## Recommended practices

1. Use consistent `/24` or `/32` addressing plans per POP
2. Centralize authentication (RADIUS) — keys still stored under `/etc/qosnat2/`
3. Monitor `wg show` peer counts and conntrack on busy nodes
4. For >5k concurrent peers on one gateway, validate on target hardware ([LOAD-TEST.md](./LOAD-TEST.md))

## HA

WG sessions do not migrate between nodes. On POP failure, clients reconnect to another POP with new configs or DNS failover.
