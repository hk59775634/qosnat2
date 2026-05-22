#!/usr/bin/env bash
# P3 冒烟：netplan 预览、DHCPv6 校验、只读 403
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8080}"
ENV_FILE="${ENV_FILE:-/etc/qosnat2/env}"
[ -f "$ENV_FILE" ] && source "$ENV_FILE" 2>/dev/null || true
PASS="${ADMIN_PASS:-${QOSNAT_PASS:-}}"

ok() { echo "OK   $*"; }
bad() { echo "FAIL $*"; exit 1; }

J=$(mktemp)
CODE=$(curl -s -o /dev/null -w "%{http_code}" -c "$J" -H 'Content-Type: application/json' \
  -d "{\"user\":\"${ADMIN_USER:-admin}\",\"pass\":\"$PASS\"}" "$BASE/api/v1/login")
[[ "$CODE" == "200" ]] || bad "login $CODE"

curl -sf -b "$J" "$BASE/api/v1/network/netplan" | grep -q '"yaml"' && ok "netplan preview"

BODY='{"enabled":true,"interface":"lo","range_start":"10.0.0.1","range_end":"10.0.0.2","router":"10.0.0.1","ipv6_enabled":true,"ipv6_prefix":"bad"}'
CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$J" -X PUT -H 'Content-Type: application/json' \
  -d "$BODY" "$BASE/api/v1/dhcp")
[[ "$CODE" == "400" ]] || bad "dhcpv6 bad prefix expected 400 got $CODE"
ok "dhcpv6 prefix validation"

curl -sf -b "$J" "$BASE/api/v1/dhcp" | grep -q '"leases"' && ok "dhcp leases json"

CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$J" -X POST -H 'Content-Type: application/json' \
  -d '{"name":"x"}' "$BASE/api/v1/shaper/wizard")
[[ "$CODE" == "403" ]] && bad "admin should not get 403 on wizard"

# 只读：仅当 state 配置了 readonly 且密码已知时测试
if [[ -n "${READONLY_PASS:-}" ]]; then
  J2=$(mktemp)
  curl -sf -c "$J2" -H 'Content-Type: application/json' \
    -d "{\"user\":\"${READONLY_USER:-viewer}\",\"pass\":\"$READONLY_PASS\"}" "$BASE/api/v1/login" >/dev/null
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$J2" -X POST -d '{}' "$BASE/api/v1/dhcp/apply")
  [[ "$CODE" == "403" ]] || bad "readonly POST expected 403 got $CODE"
  ok "readonly blocks POST"
else
  echo "SKIP readonly (set READONLY_USER/READONLY_PASS)"
fi

ok "P3 smoke done"
