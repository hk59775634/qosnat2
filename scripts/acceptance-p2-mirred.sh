#!/usr/bin/env bash
# P2：添加 100.64.0.0/24 profile 后 LAN ingress 应出现对应 u32 mirred
set -euo pipefail

PORT="${ADMIN_PORT:-8080}"
BASE="http://127.0.0.1:${PORT}"
CIDR="${TEST_CIDR:-100.64.0.0/24}"
STATE="${STATE_FILE:-/var/lib/qosnat2/state.json}"
DEV_LAN="${DEV_LAN:-ens19}"

ok() { echo "OK   $*"; }
bad() { echo "FAIL $*"; exit 1; }
skip() { echo "SKIP $*"; exit 0; }

[[ -f "$STATE" ]] && grep -q '"setup_complete": true' "$STATE" || skip "setup 未完成"
if [[ -f /etc/qosnat2/env ]]; then set -a; source /etc/qosnat2/env; set +a; fi
PASS="${QOSNAT_PASS:-${ADMIN_PASS:-}}"
[[ -n "$PASS" ]] || skip "需要 ADMIN_PASS"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
CODE=$(curl -s -o /dev/null -w "%{http_code}" -c "$TMP/j" -H 'Content-Type: application/json' \
  -d "{\"user\":\"${ADMIN_USER:-admin}\",\"pass\":\"$PASS\"}" "$BASE/api/v1/login")
[[ "$CODE" == "200" ]] || bad "login $CODE"

BODY=$(python3 -c "import json; print(json.dumps({'cidr':'$CIDR','down':'8mbit','up':'8mbit','mask':24}))")
CODE=$(curl -s -o "$TMP/out.json" -w "%{http_code}" -b "$TMP/j" -X PUT \
  -H 'Content-Type: application/json' -d "$BODY" "$BASE/api/v1/shaper/profiles")
[[ "$CODE" == "200" ]] || bad "PUT profile $CODE: $(cat "$TMP/out.json")"

sleep 2
TC=$(tc filter show dev "$DEV_LAN" ingress 2>/dev/null || true)
echo "$TC" | grep -q 'mirred' || bad "ingress 无 mirred"
# 100.64.0.0/24 → tc 显示为 match 64400000/ffffff00（src 网络序）
case "$CIDR" in
  100.64.0.0/24) echo "$TC" | grep -q '64400000' || bad "未找到 $CIDR u32: $TC" ;;
  *) echo "$TC" | grep -q 'mirred' || bad "未找到 $CIDR mirred: $TC" ;;
esac
ok "mirred 已覆盖 $CIDR"

# 清理：删除测试 profile（若原本不存在）
if ! grep -q "\"$CIDR\"" "$STATE" 2>/dev/null; then
  curl -s -o /dev/null -b "$TMP/j" -X DELETE "$BASE/api/v1/shaper/profiles?cidr=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$CIDR'))")" || true
fi
