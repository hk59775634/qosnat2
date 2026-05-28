#!/usr/bin/env bash
# qosnat2 Web UI API 冒烟测试（需已运行 qosnatd）
set -euo pipefail
BASE="${BASE:-http://127.0.0.1:8080}"
USER="${ADMIN_USER:-admin}"
PASS="${ADMIN_PASS:-${QOSNAT_PASS:-password}}"
COOKIE=$(mktemp)
trap 'rm -f "$COOKIE"' EXIT

# 自签 HTTPS 需 -k（与 acceptance 脚本一致）
CURL_K=()
[[ "$BASE" == https:* ]] && CURL_K=(-k)

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
    curl "${CURL_K[@]}" -s -o /tmp/out.json -w "%{http_code}" -b "$COOKIE" -c "$COOKIE" "$BASE$path"
  elif [ "$method" = DELETE ]; then
    curl "${CURL_K[@]}" -s -o /tmp/out.json -w "%{http_code}" -b "$COOKIE" -X DELETE "$BASE$path"
  else
    curl "${CURL_K[@]}" -s -o /tmp/out.json -w "%{http_code}" -b "$COOKIE" -c "$COOKIE" \
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
        curl "${CURL_K[@]}" -s -o /tmp/out.json -w "%{http_code}" -H "X-API-Key: $QOSNAT_API_KEY" "$BASE$path"
      elif [ "$method" = DELETE ]; then
        curl "${CURL_K[@]}" -s -o /tmp/out.json -w "%{http_code}" -H "X-API-Key: $QOSNAT_API_KEY" -X DELETE "$BASE$path"
      else
        curl "${CURL_K[@]}" -s -o /tmp/out.json -w "%{http_code}" -H "X-API-Key: $QOSNAT_API_KEY" \
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
  GET:/api/v1/setup/status:200 \
  GET:/api/v1/setup/interfaces:200 \
  GET:/api/v1/session:200 \
  GET:/api/v1/stats/dashboard:200 \
  GET:/api/v1/nat/policy-routes:200 \
  GET:/api/v1/nat/shared-ips:200 \
  GET:/api/v1/nat/static-mappings:200 \
  GET:/api/v1/nat/prefix-mappings:200 \
  GET:/api/v1/nat/wan-forwards:200 \
  GET:/api/v1/nat:200 \
  GET:/api/v1/nat/nptv6:200 \
  GET:/api/v1/nat/nat64:200 \
  GET:/api/v1/nat/dns64:200 \
  GET:/api/v1/shaper/profiles:200 \
  GET:/api/v1/shaper/active:200 \
  GET:/api/v1/routes:200 \
  GET:/api/v1/dhcp:200 \
  GET:/api/v1/vpn/wireguard/instances:200 \
  GET:/api/v1/vpn/ocserv:200 \
  GET:/api/v1/vpn/ocserv/vhosts:200 \
  GET:/api/v1/vpn/ocserv/groups:200 \
  GET:/api/v1/vpn/ocserv/install/status:200 \
  GET:/api/v1/diagnostics/captures:200 \
  GET:/api/v1/diagnostics/conntrack?limit=5:200 \
  GET:/api/v1/ebpf/maps:200 \
  GET:/api/v1/ebpf/programs:200 \
  GET:/api/v1/system/mark-policy:200 \
  GET:/api/v1/system/tuning:200 \
  GET:/api/v1/interfaces/queues:200 \
  GET:/api/v1/interfaces:200 \
  GET:/api/v1/system/general:200 \
  GET:/api/v1/system/version:200 \
  GET:/api/v1/system/audit:200 \
  GET:/api/v1/firewall/rules:200 \
  GET:/api/v1/network/vlans:200 \
  GET:/api/v1/network/wan-links:200 \
  GET:/api/v1/network/egress-policies:200 \
  GET:/api/v1/network/warp/status:200 \
  GET:/api/v1/shaper/tc:200 \
  GET:/api/v1/api-keys:200 \
  GET:/api/v1/network/vxlan:200 \
  GET:/api/v1/network/netplan:200 \
  GET:/api/v1/firewall/aliases:200 \
  GET:/api/v1/shaper/tenants:200
do
  IFS=: read -r method path expect <<< "$ep"
  code=$(req "$method" "$path")
  check "$path" "$expect" "$code"
done

echo "=== optional ocserv detail (200 or 503) ==="
code=$(req GET /api/v1/vpn/ocserv/status/detail)
if [ "$code" = "200" ] || [ "$code" = "503" ]; then ok=$((ok + 1)); echo "OK   /api/v1/vpn/ocserv/status/detail ($code)"; else fail=$((fail + 1)); echo "FAIL /api/v1/vpn/ocserv/status/detail ($code)"; fi
code=$(req GET /api/v1/vpn/ocserv/sessions)
if [ "$code" = "200" ] || [ "$code" = "503" ]; then ok=$((ok + 1)); echo "OK   /api/v1/vpn/ocserv/sessions ($code)"; else fail=$((fail + 1)); echo "FAIL /api/v1/vpn/ocserv/sessions ($code)"; fi
code=$(req GET '/api/v1/vpn/ocserv/users/traffic?username=_none_&period=24h')
if [ "$code" = "200" ]; then ok=$((ok + 1)); echo "OK   ocserv users/traffic ($code)"; else fail=$((fail + 1)); echo "FAIL ocserv users/traffic ($code)"; fi
code=$(req GET '/api/v1/vpn/wireguard/instances/default/peers/traffic?name=__no_such_peer__&period=24h')
if [ "$code" = "404" ]; then ok=$((ok + 1)); echo "OK   wireguard instance peers/traffic missing peer -> 404"; else fail=$((fail + 1)); echo "FAIL wireguard instance peers/traffic expected 404 got $code"; fi
code=$(req GET '/api/v1/vpn/ocserv/vhosts/users?domain=__no_such_vhost__.invalid')
if [ "$code" = "404" ]; then ok=$((ok + 1)); echo "OK   vhosts/users missing vhost -> 404"; else fail=$((fail + 1)); echo "FAIL vhosts/users expected 404 got $code"; fi

echo "=== interfaces ethtool (first device) ==="
if command -v python3 >/dev/null; then
  code=$(req GET /api/v1/interfaces)
  dev=$(python3 -c "import json; d=json.load(open('/tmp/out.json')); print((d.get('interfaces') or [{}])[0].get('name','') or '')" 2>/dev/null || true)
  if [ -n "$dev" ]; then
    code=$(req GET "/api/v1/interfaces/ethtool?device=${dev}")
    check "ethtool?device=$dev" 200 "$code"
  else
    echo "SKIP ethtool (no interface in GET /interfaces)"
  fi
else
  echo "SKIP ethtool (no python3)"
fi

echo "=== NAT v6 (disabled, no jool required) ==="
code=$(req PUT /api/v1/nat/nptv6 '{"nptv6_enabled":false,"nptv6_rules":[]}')
check nat-nptv6-off 200 "$code"
code=$(req PUT /api/v1/nat/nat64 '{"nat64_enabled":false,"dns64":{"mode":"local_unbound"}}')
if [ "$code" = "200" ] || [ "$code" = "500" ]; then
  ok=$((ok + 1)); echo "OK   nat64-off ($code)"
else
  fail=$((fail + 1)); echo "FAIL nat64-off ($code)"
fi

echo "=== NAT write ==="
code=$(req POST /api/v1/nat/shared-ips '{"ip":"203.0.113.10"}')
check shared-add 200 "$code"
code=$(req POST /api/v1/nat/wan-forwards '{"interface":"lo","ip_version":"ipv4","proto":"tcp","src_addr":"0.0.0.0/0","dst_port":19999,"redirect_ip":"127.0.0.1","redirect_port":8080,"comment":"smoke"}')
check forward-add 200 "$code"
code=$(req DELETE '/api/v1/nat/wan-forwards?id=fwd-dummy' )
# may 404 if id wrong - skip

echo "=== summary: $ok passed, $fail failed ==="
[ "$fail" -eq 0 ]
