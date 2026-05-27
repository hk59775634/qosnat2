#!/usr/bin/env bash
# qosnat2 验收辅助：可在网关本机运行（部分项需内网客户端配合）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STATE="${STATE_FILE:-/var/lib/qosnat2/state.json}"
PASS=0
FAIL=0

ok() { echo "OK   $1"; PASS=$((PASS + 1)); }
bad() { echo "FAIL $1"; FAIL=$((FAIL + 1)); }

echo "=== qosnat2 acceptance-check ==="

if systemctl is-active --quiet qosnatd 2>/dev/null; then
  ok "qosnatd active"
else
  bad "qosnatd not active"
fi

if curl -sf http://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then
  ok "health HTTP"
elif curl -skf https://127.0.0.1:8080/api/v1/health >/dev/null 2>&1; then
  ok "health HTTPS"
else
  bad "health HTTP/HTTPS"
fi

if [ -f "$STATE" ] && grep -q '"setup_complete": true' "$STATE"; then
  ok "setup_complete in state"
else
  bad "setup not complete"
fi

if [ -f /etc/sysctl.d/99-qosnat2.conf ]; then
  ok "sysctl.d present"
else
  bad "sysctl.d missing"
fi

if [ -f "$STATE" ] && grep -q '"nat"' "$STATE"; then
  ok "nat state block"
else
  bad "nat block missing in state (expected after upgrade)"
fi

if command -v nft >/dev/null && nft list table inet qosnat >/dev/null 2>&1; then
  ok "nft table qosnat"
else
  bad "nft table qosnat"
fi

if ip link show ifb0 >/dev/null 2>&1; then
  ok "ifb0 exists"
else
  bad "ifb0 missing"
fi

if [ -f /usr/lib/qosnat2/classify.bpf.o ] || [ -f "$ROOT/bpf/classify.bpf.o" ]; then
  ok "bpf object present"
else
  bad "bpf object missing (make bpf)"
fi

if [ -f /sys/fs/bpf/qosnat2/profile_lpm ]; then
  ok "pinned profile_lpm (/sys/fs/bpf/qosnat2)"
elif command -v bpftool >/dev/null && bpftool map show 2>/dev/null | grep -q profile_lpm; then
  ok "bpftool profile_lpm map"
else
  bad "profile_lpm map not pinned"
fi

echo ""
echo "=== 需人工验收（§14）==="
echo "  - 内网单 IP iperf 对比 QoS 模板速率 ±5%"
echo "  - VIP /32 覆盖默认网段速率"
echo "  - 重启 qosnatd 后 bpftool map 与 state.json 一致"
echo ""
echo "summary: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ]
