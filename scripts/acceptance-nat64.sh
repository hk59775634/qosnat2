#!/usr/bin/env bash
# NAT64 / NPTv6 / DNS64 验收（网关本机；需 root 与可选 jool/unbound）
set -euo pipefail

API="${QOSNAT_API:-http://127.0.0.1:8080}"
STATE="${STATE_FILE:-/var/lib/qosnat2/state.json}"
PASS=0
FAIL=0
SKIP=0

ok() { echo "OK   $1"; PASS=$((PASS + 1)); }
bad() { echo "FAIL $1"; FAIL=$((FAIL + 1)); }
skip() { echo "SKIP $1"; SKIP=$((SKIP + 1)); }

echo "=== qosnat2 acceptance-nat64 ==="

if ! command -v curl >/dev/null; then
  bad "curl required"
  exit 1
fi

if curl -sf "${API}/api/v1/health" >/dev/null 2>&1; then
  ok "health"
else
  bad "health ${API}"
fi

if [ -f "$STATE" ] && grep -q '"nat"' "$STATE"; then
  ok "state has nat block"
else
  bad "state missing nat (migrate or save via API)"
fi

if command -v nft >/dev/null && nft list table inet qosnat >/dev/null 2>&1; then
  ok "nft table qosnat"
  if grep -q '"nptv6_enabled": true' "$STATE" 2>/dev/null; then
    if nft list chain inet qosnat postrouting 2>/dev/null | grep -q 'snat ip6 prefix'; then
      ok "nptv6 snat rules present"
    else
      bad "nptv6 enabled in state but no snat ip6 prefix in nft"
    fi
  else
    skip "nptv6 not enabled"
  fi
else
  bad "nft table qosnat missing"
fi

if grep -q '"nat64_enabled": true' "$STATE" 2>/dev/null; then
  if command -v jool >/dev/null; then
    if jool instance display 2>/dev/null | grep -q qosnat2; then
      ok "jool instance qosnat2"
    else
      bad "jool instance qosnat2 not found"
    fi
  else
    skip "jool not installed"
  fi
  if grep -q '"mode": "local_unbound"' "$STATE" 2>/dev/null; then
    if [ -f /etc/unbound/unbound.conf.d/qosnat2-dns64.conf ]; then
      ok "unbound drop-in config"
    else
      bad "local_unbound mode but unbound config missing"
    fi
    if command -v unbound >/dev/null; then
      if systemctl is-active --quiet unbound 2>/dev/null; then
        ok "unbound active"
      else
        bad "unbound not active"
      fi
    else
      skip "unbound not installed"
    fi
  else
    skip "dns64 not local_unbound"
  fi
  if [ -f /etc/dnsmasq.d/qosnat2.conf ] && grep -q '127.0.0.1#5353' /etc/dnsmasq.d/qosnat2.conf 2>/dev/null; then
    ok "dnsmasq forwards to unbound"
  elif grep -q '"mode": "upstream"' "$STATE" 2>/dev/null; then
    ok "dnsmasq upstream dns64 mode (no local forward expected)"
  else
    skip "dnsmasq forward check"
  fi
else
  skip "nat64 not enabled in state"
fi

echo "---"
echo "PASS=$PASS FAIL=$FAIL SKIP=$SKIP"
[ "$FAIL" -eq 0 ]
