# Operations backup

Back up qosnat2 configuration before upgrades or risky changes.

## state.json

Primary configuration lives at `/var/lib/qosnat2/state.json` (override with `STATE_FILE`).

Example daily cron (run as root):

```bash
0 3 * * * cp -a /var/lib/qosnat2/state.json /var/backups/qosnat2/state-$(date +\%F).json
```

Keep at least 7 daily copies off-box (rsync, S3, etc.). The daemon also writes `state.json.bak` on each atomic save (single generation).

## Sessions

Session tokens: `/var/lib/qosnat2/sessions.json` — back up if you need users to stay logged in across restore; otherwise re-login is acceptable.

## Restore

1. Stop `qosnatd`: `systemctl stop qosnatd`
2. Replace `state.json` from backup
3. Start `qosnatd`: `systemctl start qosnatd`
4. Verify Admin UI and run **System → Export** ETag/compare if needed
5. Re-apply dataplane-sensitive pages (firewall, NAT, DHCP) if services drifted

## TLS certificates

If using ACME or uploaded certs managed by qosnat2, export `/etc/qosnat2/` TLS material separately.

## Not included in state.json

- dnsmasq runtime leases (`/var/lib/misc/dnsmasq.leases`)
- ocserv/WG key material under `/etc/qosnat2/` paths
- nft rules on disk are regenerated from state on boot

Document site-specific paths in your runbook.
