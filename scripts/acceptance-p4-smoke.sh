#!/usr/bin/env bash
# P4 冒烟：租户 API、VXLAN 预览、ASN alias 校验
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8080}"
[ -f /etc/qosnat2/env ] && source /etc/qosnat2/env
PASS="${ADMIN_PASS:-}"
J=$(mktemp)
trap 'rm -f "$J"' EXIT
curl -sf -c "$J" -H 'Content-Type: application/json' \
  -d "{\"user\":\"${ADMIN_USER:-admin}\",\"pass\":\"$PASS\"}" "$BASE/api/v1/login" >/dev/null

curl -sf -b "$J" "$BASE/api/v1/shaper/tenants" | grep -q '"tenants"' && echo "OK   tenants list"

BODY='{"name":"test-tenant","cidrs":["10.99.0.0/24"],"down":"8mbit","up":"8mbit"}'
RESP=$(curl -sf -b "$J" -X POST -H 'Content-Type: application/json' -d "$BODY" "$BASE/api/v1/shaper/tenants")
ID=$(python3 -c "import json,sys; print(json.load(sys.stdin)['id'])" <<<"$RESP")
curl -sf -b "$J" "$BASE/api/v1/shaper/profiles" | grep -q '10.99.0.0/24' && echo "OK   tenant profile expanded"
curl -sf -b "$J" -X DELETE "$BASE/api/v1/shaper/tenants?id=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$ID'))")" >/dev/null && echo "OK   tenant cleanup"

curl -sf -b "$J" "$BASE/api/v1/network/netplan" | grep -q '"yaml"' && echo "OK   netplan preview"

CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$J" -X POST -H 'Content-Type: application/json' \
  -d '{"name":"bad-asn","type":"asn","members":["10.0.0.0/8"]}' "$BASE/api/v1/firewall/aliases")
[[ "$CODE" == "400" ]] && echo "OK   asn alias requires asn number"

echo "OK   P4 smoke done"
