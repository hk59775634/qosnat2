#!/usr/bin/env bash
# ocserv vhost API 冒烟：创建 / 独立 RADIUS / 更新 / 删除
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8080}"
[ -f /etc/qosnat2/env ] && source /etc/qosnat2/env
PASS="${ADMIN_PASS:-}"
DOMAIN="smoke-vhost-$(date +%s).test"
J=$(mktemp)
trap 'rm -f "$J"' EXIT

curl -skf -c "$J" -H 'Content-Type: application/json' \
  -d "{\"user\":\"${ADMIN_USER:-admin}\",\"pass\":\"$PASS\"}" "$BASE/api/v1/login" >/dev/null

curl -skf -b "$J" "$BASE/api/v1/vpn/ocserv" | grep -q '"vhosts"' && echo "OK   ocserv GET"

BODY=$(cat <<EOF
{"enabled":true,"domain":"$DOMAIN","comment":"smoke","auth_method":"radius","radius":{"server":"127.0.0.1","auth_port":1812,"acct_port":1813,"secret":"smoke-secret","groupconfig":true}}
EOF
)
CODE=$(curl -sk -o /dev/null -w "%{http_code}" -b "$J" -X POST -H 'Content-Type: application/json' \
  -d "$BODY" "$BASE/api/v1/vpn/ocserv/vhosts")
[[ "$CODE" == "200" ]] && echo "OK   vhost POST radius dedicated"

curl -skf -b "$J" "$BASE/api/v1/vpn/ocserv/vhosts" | grep -q "$DOMAIN" && echo "OK   vhost list"

PUT=$(cat <<EOF
{"enabled":true,"domain":"$DOMAIN","comment":"smoke-updated","auth_method":"radius","radius":{"server":"127.0.0.2","auth_port":1812,"acct_port":1813,"secret":"smoke-secret","groupconfig":false}}
EOF
)
CODE=$(curl -sk -o /dev/null -w "%{http_code}" -b "$J" -X PUT -H 'Content-Type: application/json' \
  -d "$PUT" "$BASE/api/v1/vpn/ocserv/vhosts")
[[ "$CODE" == "200" ]] && echo "OK   vhost PUT update"

ENC=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$DOMAIN'))")
CODE=$(curl -sk -o /dev/null -w "%{http_code}" -b "$J" -X DELETE "$BASE/api/v1/vpn/ocserv/vhosts?domain=$ENC")
[[ "$CODE" == "200" ]] && echo "OK   vhost DELETE"

! curl -skf -b "$J" "$BASE/api/v1/vpn/ocserv/vhosts" | grep -q "$DOMAIN" && echo "OK   vhost removed"

echo "OK   ocserv vhost smoke done"
