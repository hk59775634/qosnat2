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

GLOBAL_NET=$(curl -skf -b "$J" "$BASE/api/v1/vpn/ocserv" | python3 -c "import sys,json; print(json.load(sys.stdin)['config']['ipv4_network'])")
VHOST_NET=$(curl -skf -b "$J" "$BASE/api/v1/vpn/ocserv" | python3 -c "import sys,json; d=json.load(sys.stdin); v=[x for x in d['config']['vhosts'] if x['domain']=='$DOMAIN']; print(v[0]['ipv4_network'] if v else '')")
[[ -n "$GLOBAL_NET" && "$VHOST_NET" == "$GLOBAL_NET" ]] && echo "OK   vhost inherits global ipv4_network"

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

PLAIN_DOMAIN="smoke-plain-$(date +%s).test"
PASSWD="/tmp/smoke-vhost-${PLAIN_DOMAIN}.passwd"
PUT_PLAIN=$(cat <<EOF
{"enabled":true,"domain":"$PLAIN_DOMAIN","comment":"smoke-plain","auth_method":"plain","plain_passwd_path":"$PASSWD"}
EOF
)
CODE=$(curl -sk -o /dev/null -w "%{http_code}" -b "$J" -X POST -H 'Content-Type: application/json' \
  -d "$PUT_PLAIN" "$BASE/api/v1/vpn/ocserv/vhosts")
[[ "$CODE" == "200" ]] && echo "OK   vhost POST plain passwd path"

ENC_PLAIN=$(python3 -c "import urllib.parse; print(urllib.parse.quote('$PLAIN_DOMAIN'))")
CODE=$(curl -sk -o /dev/null -w "%{http_code}" -b "$J" -X POST -H 'Content-Type: application/json' \
  -d '{"username":"smokeuser","password":"smokepass","comment":"t","group":""}' \
  "$BASE/api/v1/vpn/ocserv/vhosts/users?domain=$ENC_PLAIN")
[[ "$CODE" == "200" ]] && echo "OK   vhost plain user POST"

curl -skf -b "$J" "$BASE/api/v1/vpn/ocserv/vhosts/users?domain=$ENC_PLAIN" | grep -q smokeuser && echo "OK   vhost plain user GET"

CODE=$(curl -sk -o /dev/null -w "%{http_code}" -b "$J" -X DELETE \
  "$BASE/api/v1/vpn/ocserv/vhosts?domain=$ENC_PLAIN")
[[ "$CODE" == "200" ]] && echo "OK   vhost plain DELETE"
rm -f "$PASSWD"

echo "OK   ocserv vhost smoke done"
