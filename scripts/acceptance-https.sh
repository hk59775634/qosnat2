#!/usr/bin/env bash
# P1 HTTPS 验收：自签证书 → API 启用 TLS → HTTPS health + 登录
set -euo pipefail

PORT="${ADMIN_PORT:-8080}"
BASE_HTTP="http://127.0.0.1:${PORT}"
BASE_HTTPS="https://127.0.0.1:${PORT}"
PASS="${QOSNAT_PASS:-${ADMIN_PASS:-}}"
STATE="${STATE_FILE:-/var/lib/qosnat2/state.json}"
# 验收结束后恢复 HTTP（避免占用 8080 仅 HTTPS）；设 RESTORE_HTTP=0 可保留 HTTPS
RESTORE_HTTP="${RESTORE_HTTP:-1}"

ok() { echo "OK   $*"; }
bad() { echo "FAIL $*"; exit 1; }
skip() { echo "SKIP $*"; exit 0; }

[[ "$(id -u)" -eq 0 ]] || bad "请用 root/sudo 运行（写证书与 restart）"
[[ -f "$STATE" ]] && grep -q '"setup_complete": true' "$STATE" || skip "setup_complete 为 false，先完成向导"

if [[ -z "$PASS" ]] && [[ -f /etc/qosnat2/env ]]; then
  set -a
  # shellcheck source=/dev/null
  source /etc/qosnat2/env
  set +a
  PASS="${ADMIN_PASS:-}"
fi
[[ -n "$PASS" ]] || skip "设置 QOSNAT_PASS 或 ADMIN_PASS"

command -v openssl >/dev/null || bad "需要 openssl"
command -v python3 >/dev/null || bad "需要 python3"

TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
openssl req -x509 -newkey rsa:2048 -keyout "$TMP/tls.key" -out "$TMP/tls.crt" -days 30 -nodes \
  -subj /CN=qosnat2-test 2>/dev/null

PUT_BODY=$(python3 - "$TMP/tls.crt" "$TMP/tls.key" "$PASS" <<'PY'
import json, sys
cert = open(sys.argv[1]).read()
key = open(sys.argv[2]).read()
pw = sys.argv[3]
print(json.dumps({
    "tls_enabled": True,
    "tls_cert": cert,
    "tls_key": key,
    "current_password": pw,
}))
PY
)

COOKIE_JAR="$TMP/cookies.txt"
CODE=$(curl -s -o "$TMP/login.json" -w "%{http_code}" -c "$COOKIE_JAR" \
  -H 'Content-Type: application/json' \
  -d "{\"user\":\"${ADMIN_USER:-admin}\",\"pass\":\"$PASS\"}" \
  "$BASE_HTTP/api/v1/login")
[[ "$CODE" == "200" ]] || bad "HTTP login $CODE"

echo "$PUT_BODY" >"$TMP/tls-put.json"
CODE=$(curl -s -o "$TMP/tls.json" -w "%{http_code}" -b "$COOKIE_JAR" -X PUT \
  -H 'Content-Type: application/json' --data-binary "@$TMP/tls-put.json" \
  "$BASE_HTTP/api/v1/system/general")
[[ "$CODE" == "200" ]] || bad "PUT general TLS $CODE: $(cat "$TMP/tls.json")"

echo "等待 qosnatd 重启…"
sleep 5
if curl -skf "$BASE_HTTPS/api/v1/health" | grep -q '"tls_active":true'; then
  ok "HTTPS health tls_active"
else
  bad "HTTPS health 未显示 tls_active（检查 systemctl status qosnatd）"
fi

CODE=$(curl -sk -o "$TMP/login2.json" -w "%{http_code}" -c "$TMP/c2.txt" \
  -H 'Content-Type: application/json' \
  -d "{\"user\":\"${ADMIN_USER:-admin}\",\"pass\":\"$PASS\"}" \
  "$BASE_HTTPS/api/v1/login")
[[ "$CODE" == "200" ]] || bad "HTTPS login $CODE"

ok "HTTPS 验收通过（Cookie 登录）"

if [[ "$RESTORE_HTTP" == "1" ]]; then
  PUT_OFF=$(python3 - -c "$PASS" <<'PY'
import json, sys
print(json.dumps({"tls_enabled": False, "current_password": sys.argv[1]}))
PY
)
  curl -sk -o /dev/null -b "$TMP/c2.txt" -X PUT -H 'Content-Type: application/json' \
    -d "$PUT_OFF" "$BASE_HTTPS/api/v1/system/general" 2>/dev/null || true
  sleep 2
  ok "已恢复 HTTP（RESTORE_HTTP=1）"
fi
