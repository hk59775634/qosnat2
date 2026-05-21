#!/usr/bin/env bash
# qosnat2 Web UI API 冒烟测试（需已运行 qosnatd）
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8080}"
USER="${ADMIN_USER:-admin}"
PASS="${ADMIN_PASS:-${QOSNAT_PASS:-QosNat@2026}}"
COOKIE=$(mktemp)
trap 'rm -f "$COOKIE"' EXIT

fail=0
ok=0

check() {
  local name="$1" expect="$2" code="$3"
  if [ "$code" = "$expect" ]; then
    echo "OK   $name ($code)"
    ok=$((ok + 1))
  else
    echo "FAIL $name (got $code want $expect) $(head -c 80 /tmp/out.json)"
    fail=$((fail + 1))
  fi
}

req() {
  local method="$1" path="$2" data="${3:-}"
  if [ "$method" = GET ]; then
    curl -s -o /tmp/out.json -w "%{http_code}" -b "$COOKIE" -c "$COOKIE" "$BASE$path"
  elif [ "$method" = DELETE ]; then
    curl -s -o /tmp/out.json -w "%{http_code}" -b "$COOKIE" -X DELETE "$BASE$path"
  else
    curl -s -o /tmp/out.json -w "%{http_code}" -b "$COOKIE" -c "$COOKIE" \
      -X "$method" -H 'Content-Type: application/json' -d "$data" "$BASE$path"
  fi
}

echo "=== setup ==="
code=$(req GET /api/v1/setup/status)
check setup-status 200 "$code"
if grep -q '"setup_required":true' /tmp/out.json 2>/dev/null; then
  echo "SKIP setup required — run Web wizard or set DEV_LAN/DEV_WAN in env"
  exit 0
fi

echo "=== login ==="
code=$(req POST /api/v1/login "{\"user\":\"$USER\",\"pass\":\"$PASS\"}")
check login 200 "$code"
if [ "$code" != "200" ]; then
  if [ -n "${QOSNAT_API_KEY:-}" ]; then
    echo "WARN login failed — 使用 QOSNAT_API_KEY 请求头继续"
    req() {
      local method="$1" path="$2" data="${3:-}"
      if [ "$method" = GET ]; then
        curl -s -o /tmp/out.json -w "%{http_code}" -H "X-API-Key: $QOSNAT_API_KEY" "$BASE$path"
      elif [ "$method" = DELETE ]; then
        curl -s -o /tmp/out.json -w "%{http_code}" -H "X-API-Key: $QOSNAT_API_KEY" -X DELETE "$BASE$path"
      else
        curl -s -o /tmp/out.json -w "%{http_code}" -H "X-API-Key: $QOSNAT_API_KEY" \
          -X "$method" -H 'Content-Type: application/json' -d "$data" "$BASE$path"
      fi
    }
  else
    echo "提示: 设置 ADMIN_PASS / QOSNAT_PASS，或创建 API Key 后 export QOSNAT_API_KEY=..."
    exit 1
  fi
fi

echo "=== read APIs (UI pages) ==="
for ep in \
  GET:/api/v1/health:200 \
  GET:/api/v1/session:200 \
  GET:/api/v1/stats/dashboard:200 \
  GET:/api/v1/nat/policy-routes:200 \
  GET:/api/v1/nat/shared-ips:200 \
  GET:/api/v1/nat/static-mappings:200 \
  GET:/api/v1/nat/prefix-mappings:200 \
  GET:/api/v1/nat/wan-forwards:200 \
  GET:/api/v1/shaper/profiles:200 \
  GET:/api/v1/shaper/active:200 \
  GET:/api/v1/routes:200 \
  GET:/api/v1/dhcp:200 \
  GET:/api/v1/vpn/wireguard:200 \
  GET:/api/v1/diagnostics/captures:200 \
  GET:/api/v1/diagnostics/conntrack?limit=5:200 \
  GET:/api/v1/ebpf/maps:200 \
  GET:/api/v1/ebpf/programs:200 \
  GET:/api/v1/system/mark-policy:200 \
  GET:/api/v1/system/tuning:200 \
  GET:/api/v1/interfaces/queues:200 \
  GET:/api/v1/interfaces:200 \
  GET:/api/v1/shaper/hosts:200 \
  GET:/api/v1/system/general:200 \
  GET:/api/v1/system/audit:200 \
  GET:/api/v1/firewall/rules:200 \
  GET:/api/v1/firewall/geoip:200 \
  GET:/api/v1/network/vlans:200 \
  GET:/api/v1/network/wan-links:200 \
  GET:/api/v1/shaper/tc:200 \
  GET:/api/v1/api-keys:200
do
  IFS=: read -r method path expect <<< "$ep"
  code=$(req "$method" "$path")
  check "$path" "$expect" "$code"
done

echo "=== NAT write ==="
code=$(req POST /api/v1/nat/shared-ips '{"ip":"203.0.113.10"}')
check shared-add 200 "$code"
code=$(req POST /api/v1/nat/wan-forwards '{"interface":"lo","ip_version":"ipv4","proto":"tcp","src_addr":"0.0.0.0/0","dst_port":19999,"redirect_ip":"127.0.0.1","redirect_port":8080,"comment":"smoke"}')
check forward-add 200 "$code"
code=$(req DELETE '/api/v1/nat/wan-forwards?id=fwd-dummy' )
# may 404 if id wrong - skip

echo "=== summary: $ok passed, $fail failed ==="
[ "$fail" -eq 0 ]
