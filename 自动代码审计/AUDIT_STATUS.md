# Audit remediation status

**Updated**: 2026-05-30  
**Baseline commits**: `86346ef` (core audit fixes), `2997383` (DHCP/DNS split), plus follow-up in working tree.

## Completed

| ID | Item | Notes |
|----|------|-------|
| F-001 | Scoped nft delete table | `internal/nft`, deploy scripts |
| F-002-A | Terminal default off | `DiagnosticsTerminalEnabled` |
| F-002-B | Terminal re-auth grant | `POST /diagnostics/terminal/grant` + UI modal |
| F-002-D | Terminal CIDR allowlist | `QOSNAT_TERMINAL_ALLOW_CIDRS` |
| F-003/006/007 | NAT/egress apply pipeline | revert helpers |
| F-004-B | applyNatStack rollback | `lastNatStackSnapshot` + step rollback |
| F-005 | nft apply mutex | `withNftApply` |
| F-009/010 | Atomic state write | `.bak` generation |
| F-011 | Save error handling | `persistState`, batch handler fixes |
| F-012 | Shaper profile upsert | prior commit |
| F-014 | Firewall dry-run UI | prior commit |
| F-015/016 | API error envelope | `writeAPIError` + code field (incremental on handlers) |
| F-017 | API key RBAC | admin / readonly / firewall scopes |
| F-018–F-020 | Terminal warning, search, state export/import | UI + handlers |
| F-021 | Terminal Origin check | same-host only |
| F-022 | OpenAPI route check | `scripts/check-openapi-routes.sh` in CI |
| F-023 | ETag | `GET /system/state/export` |
| F-024 | Server split | documented as future refactor in ARCHITECTURE_AUDIT |
| F-025 | Firewall search | prior commit |
| F-026 | CI nft smoke | acceptance job |
| F-027 | WG scaling docs | `docs/WIREGUARD-SCALING.md` |
| F-028 | URL naming | REST paths stable; breaking rename deferred |
| Ops | Backup / load test / HA | `docs/OPS-BACKUP.md`, `LOAD-TEST.md`, `HA-DEPLOYMENT.md` |
| DHCP/DNS | Independent modes + upstream | `2997383` |

## Deferred (product roadmap, not blockers)

| Item | Reason |
|------|--------|
| F-002-C Terminal降权 (nobody/rbash) | Optional hardening; grant + default-off sufficient for audit close |
| Full session RBAC + nav `v-if` | API keys cover automation; UI roles need product design |
| nft full incremental | env `QOSNAT_NFT_INCREMENTAL` partial filter ops only |
| UI mid-term items | Rule wizard, nft diff preview, optimistic lock |
| 100k user single-node | Architecture doc — multi-POP required |

## Verification

```bash
go test ./internal/api/... ./internal/store/... ./internal/dnsmasq/...
bash scripts/check-openapi-routes.sh
cd web && npm run build
```

Production: rebuild `qosnatd`, restart service after pulling changes.
