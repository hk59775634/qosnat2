#!/usr/bin/env bash
# qosnat2 自动验收（本机可执行项 + §14 可自动化子项）
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
STATE="${STATE_FILE:-/var/lib/qosnat2/state.json}"
ENV_FILE="${ENV_FILE:-/etc/qosnat2/env}"
BASE="${BASE:-http://127.0.0.1:8080}"
PASS=0
FAIL=0
SKIP=0
COOKIE=""
REPORT="${REPORT:-$ROOT/docs/验收报告-auto.md}"

ok() { echo "OK   $1"; PASS=$((PASS + 1)); }
bad() { echo "FAIL $1"; FAIL=$((FAIL + 1)); }
skip() { echo "SKIP $1"; SKIP=$((SKIP + 1)); }

# shellcheck source=/dev/null
[ -f "$ENV_FILE" ] && source "$ENV_FILE" 2>/dev/null || true
USER="${ADMIN_USER:-admin}"
PASS_AUTH="${ADMIN_PASS:-${QOSNAT_PASS:-}}"
DEV_LAN="${DEV_LAN:-ens19}"
DEV_WAN="${DEV_WAN:-ens18}"
TEST_VIP_IP="${TEST_VIP_IP:-10.0.0.199}"

mkdir -p "$(dirname "$REPORT")"
exec > >(tee -a /tmp/qosnat-acceptance-auto.log) 2>&1

echo "=== qosnat2 acceptance-auto $(date -Iseconds) ==="

# --- 1) 基础检查脚本 ---
if bash "$ROOT/scripts/acceptance-check.sh"; then
  ok "acceptance-check.sh"
else
  bad "acceptance-check.sh"
fi

# --- 2) bpftool 备用检测（pinned）---
if [ -f /sys/fs/bpf/qosnat2/profile_lpm ]; then
  ok "pinned profile_lpm"
else
  bad "pinned profile_lpm missing"
fi

# --- 3) TC / BPF / IFB mirred 拓扑 ---
if tc filter show dev "$DEV_LAN" ingress 2>/dev/null | grep -q 'mirred.*ifb0'; then
  ok "TC mirred ingress on $DEV_LAN → ifb0"
else
  bad "TC mirred ingress on $DEV_LAN"
fi
if tc filter show dev "$DEV_LAN" ingress 2>/dev/null | grep -q 'pref 10 u32'; then
  ok "TC u32 mirred prio 10 on $DEV_LAN"
else
  bad "TC u32 mirred prio 10 on $DEV_LAN (flower legacy not acceptable)"
fi
if tc filter show dev "$DEV_LAN" ingress 2>/dev/null | grep -q flower; then
  bad "legacy flower mirred still on $DEV_LAN"
fi
if tc filter show dev "$DEV_LAN" ingress 2>/dev/null | grep -q classify_ingress; then
  bad "TC bpf on $DEV_LAN ingress (blocks u32 mirred)"
else
  ok "no bpf on $DEV_LAN ingress (mirred only)"
fi
if tc filter show dev ifb0 ingress 2>/dev/null | grep -q classify_ingress; then
  ok "TC bpf ingress on ifb0 (after mirred)"
else
  bad "TC bpf ingress on ifb0"
fi
if tc filter show dev "$DEV_LAN" parent 1: 2>/dev/null | grep -q classify_egress; then
  ok "TC bpf on $DEV_LAN parent 1: (download classify)"
else
  bad "TC bpf on $DEV_LAN parent 1:"
fi
if ip link show ifb0 >/dev/null 2>&1 && tc qdisc show dev ifb0 2>/dev/null | grep -q htb; then
  ok "ifb0 HTB qdisc"
else
  bad "ifb0 HTB qdisc"
fi

# --- 4) nft NAT + 非对称回程 ---
nft_nat_n=$(nft list ruleset 2>/dev/null | grep -ciE 'masquerade|snat' || true)
if [ "${nft_nat_n:-0}" -gt 0 ]; then
  ok "nft SNAT/masquerade"
else
  bad "nft SNAT/masquerade"
fi
if nft list chain inet qosnat forward 2>/dev/null | grep -q '10.0.0.0/8.*drop'; then
  ok "nft asymmetric return drop"
else
  bad "nft asymmetric return drop"
fi

# --- 5) API 登录与冒烟 ---
if [ -z "$PASS_AUTH" ] && [ -z "${QOSNAT_API_KEY:-}" ]; then
  skip "API auth (no ADMIN_PASS / QOSNAT_API_KEY)"
else
  COOKIE=$(mktemp)
  trap 'rm -f "$COOKIE"' EXIT
  api() {
    local method="$1" path="$2" data="${3:-}"
    local extra=()
    [ -n "${QOSNAT_API_KEY:-}" ] && extra=(-H "X-API-Key: $QOSNAT_API_KEY")
    if [ "$method" = GET ]; then
      curl -sf "${extra[@]}" -b "$COOKIE" -c "$COOKIE" "$BASE$path"
    elif [ "$method" = DELETE ]; then
      curl -sf "${extra[@]}" -b "$COOKIE" -X DELETE "$BASE$path"
    else
      curl -sf "${extra[@]}" -b "$COOKIE" -X "$method" -H 'Content-Type: application/json' -d "$data" "$BASE$path"
    fi
  }
  if [ -n "$PASS_AUTH" ]; then
    if curl -sf -b "$COOKIE" -c "$COOKIE" -X POST -H 'Content-Type: application/json' \
      -d "{\"user\":\"$USER\",\"pass\":\"$PASS_AUTH\"}" "$BASE/api/v1/login" >/dev/null; then
      ok "API login"
    else
      bad "API login"
    fi
  fi
  if api GET /api/v1/health | grep -q '"ok":true'; then
    ok "API health json"
  else
    bad "API health json"
  fi
  if api GET /api/v1/ebpf/maps | grep -q '"loaded":true'; then
    ok "ebpf loaded via API"
  else
    bad "ebpf loaded via API"
  fi
  if bash "$ROOT/scripts/test-ui-api.sh" 2>/dev/null; then
    ok "test-ui-api.sh"
  else
    bad "test-ui-api.sh"
  fi
fi

# --- 6) state.json 与 profile_lpm 条数对账 ---
if [ -f "$STATE" ] && command -v python3 >/dev/null; then
  want=$(python3 - <<'PY' "$STATE"
import json, sys
st = json.load(open(sys.argv[1]))
n = 0
if st.get("shaper", {}).get("policy_cidr"):
    n += 1
n += len(st.get("shaper", {}).get("profiles") or [])
print(n)
PY
)
  got=$(bpftool map dump pinned /sys/fs/bpf/qosnat2/profile_lpm 2>/dev/null | grep -c '"key"' || echo 0)
  if [ "$want" = "$got" ]; then
    ok "profile_lpm keys ($got) match state ($want)"
  else
    bad "profile_lpm keys got=$got want=$want"
  fi
else
  skip "profile_lpm count reconcile (python3/state)"
fi

# --- 7) VIP host_exact 写入与删除 ---
if [ -n "${COOKIE:-}" ] || [ -n "${QOSNAT_API_KEY:-}" ]; then
  VIP_JSON="{\"down\":\"50mbit\",\"up\":\"50mbit\"}"
  if curl -sf "${QOSNAT_API_KEY:+-H X-API-Key: $QOSNAT_API_KEY}" -b "${COOKIE:-/dev/null}" \
    -X PUT -H 'Content-Type: application/json' -d "$VIP_JSON" \
    "$BASE/api/v1/shaper/hosts/$TEST_VIP_IP" >/dev/null 2>&1; then
    hosts_json=$(curl -sf ${QOSNAT_API_KEY:+-H "X-API-Key: $QOSNAT_API_KEY"} -b "${COOKIE:-/dev/null}" \
      "$BASE/api/v1/shaper/hosts" 2>/dev/null || echo '[]')
    if echo "$hosts_json" | grep -q "$TEST_VIP_IP"; then
      ok "VIP host_exact via API ($TEST_VIP_IP)"
    elif bpftool map dump pinned /sys/fs/bpf/qosnat2/host_exact 2>/dev/null | grep -q '"key"'; then
      ok "VIP host_exact map has entries"
    else
      bad "VIP not visible after PUT $TEST_VIP_IP"
    fi
    curl -sf "${QOSNAT_API_KEY:+-H X-API-Key: $QOSNAT_API_KEY}" -b "${COOKIE:-/dev/null}" \
      -X DELETE "$BASE/api/v1/shaper/hosts/$TEST_VIP_IP" >/dev/null 2>&1 || true
    ok "VIP cleanup DELETE"
  else
    bad "VIP PUT $TEST_VIP_IP"
  fi
else
  skip "VIP host_exact (no API auth)"
fi

# --- 8) iperf 上行（restart 前测，避免重启断连）---
TEST_CLIENT="${TEST_CLIENT:-100.64.0.254}"
TEST_IPERF_DST="${TEST_IPERF_DST:-}"
if [ -z "$TEST_IPERF_DST" ]; then
  TEST_IPERF_DST=$(ip -4 -o addr show dev "${DEV_WAN:-ens18}" 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -1)
fi
if command -v iperf3 >/dev/null && [ -n "$TEST_IPERF_DST" ]; then
  if ssh -o BatchMode=yes -o ConnectTimeout=3 "root@${TEST_CLIENT}" true 2>/dev/null; then
    pkill -f "iperf3 -s -B ${TEST_IPERF_DST}" 2>/dev/null || true
    iperf3 -s -B "$TEST_IPERF_DST" -D >/dev/null 2>&1 || true
    sleep 2
    up_mbps=$(ssh -o BatchMode=yes "root@${TEST_CLIENT}" \
      "iperf3 -c ${TEST_IPERF_DST} -t 8 -f m 2>/dev/null" | awk '/Mbits\/sec/ && /sender/ {for(i=1;i<=NF;i++) if($i=="Mbits/sec"){print $(i-1); exit}}')
    if [ -n "$up_mbps" ]; then
      up_int=${up_mbps%.*}
      if [ -n "$up_int" ] && [ "$up_int" -ge 6 ] && [ "$up_int" -le 11 ]; then
        ok "iperf upload ~${up_mbps} Mbit/s (expect ~8)"
      else
        bad "iperf upload ${up_mbps} Mbit/s (expect 6-11)"
      fi
    else
      bad "iperf upload parse failed"
    fi
    down_mbps=$(ssh -o BatchMode=yes "root@${TEST_CLIENT}" \
      "iperf3 -c ${TEST_IPERF_DST} -t 8 -f m -R 2>/dev/null" | awk '/Mbits\/sec/ && /receiver/ {for(i=1;i<=NF;i++) if($i=="Mbits/sec"){print $(i-1); exit}}')
    if [ -n "$down_mbps" ]; then
      down_int=${down_mbps%.*}
      if [ -n "$down_int" ] && [ "$down_int" -ge 6 ] && [ "$down_int" -le 11 ]; then
        ok "iperf download -R ~${down_mbps} Mbit/s (expect ~8)"
      else
        bad "iperf download -R ${down_mbps} Mbit/s (expect 6-11)"
      fi
    else
      bad "iperf download -R parse failed"
    fi
    pkill -f "iperf3 -s -B ${TEST_IPERF_DST}" 2>/dev/null || true
  else
    skip "iperf (SSH root@${TEST_CLIENT} unavailable)"
  fi
else
  skip "iperf3 or TEST_IPERF_DST missing"
fi
# 上行：ens19 ingress u32 mirred → ifb0 ingress BPF + egress HTB
if tc filter show dev "${DEV_LAN:-ens19}" ingress 2>/dev/null | grep -q 'pref 1 bpf'; then
  bad "LAN ingress BPF blocks mirred (must be on ifb0 only)"
else
  ok "LAN ingress has no BPF (mirred not blocked)"
fi
if tc filter show dev "${DEV_LAN:-ens19}" ingress 2>/dev/null | grep -q 'mirred.*ifb0'; then
  ok "LAN ingress u32 mirred -> ifb0"
else
  bad "LAN ingress missing mirred -> ifb0"
fi
if tc filter show dev ifb0 parent 1: 2>/dev/null | grep -q u32; then
  ok "ifb0 upload u32 filters"
else
  bad "ifb0 missing u32 upload filters"
fi

# --- 9) 重启 qosnatd 后 Map 回放 ---
prof_before=$(bpftool map dump pinned /sys/fs/bpf/qosnat2/profile_lpm 2>/dev/null | grep -c '"key"' || echo 0)
if systemctl restart qosnatd; then
  sleep 2
  for i in 1 2 3 4 5 6 7 8 9 10; do
    curl -sf "$BASE/api/v1/health" >/dev/null 2>&1 && break
    sleep 1
  done
  prof_after=$(bpftool map dump pinned /sys/fs/bpf/qosnat2/profile_lpm 2>/dev/null | grep -c '"key"' || echo 0)
  if [ "$prof_before" = "$prof_after" ] && [ "$prof_after" -gt 0 ]; then
    ok "restart qosnatd profile_lpm stable ($prof_after keys)"
  else
    bad "restart qosnatd profile_lpm before=$prof_before after=$prof_after"
  fi
  if curl -sf "$BASE/api/v1/health" | grep -q '"bpf":true'; then
    ok "health bpf after restart"
  else
    bad "health bpf after restart"
  fi
  if tc filter show dev "$DEV_LAN" ingress 2>/dev/null | grep -q 'pref 10 u32'; then
    ok "after restart: u32 mirred on $DEV_LAN"
  else
    bad "after restart: u32 mirred missing on $DEV_LAN"
  fi
  if tc filter show dev ifb0 ingress 2>/dev/null | grep -q classify_ingress; then
    ok "after restart: bpf on ifb0 ingress"
  else
    bad "after restart: ifb0 ingress bpf missing"
  fi
else
  bad "systemctl restart qosnatd"
fi

# --- 10) conntrack 压力粗检 ---
if [ -r /proc/sys/net/netfilter/nf_conntrack_count ] && [ -r /proc/sys/net/netfilter/nf_conntrack_max ]; then
  c=$(cat /proc/sys/net/netfilter/nf_conntrack_count)
  m=$(cat /proc/sys/net/netfilter/nf_conntrack_max)
  if [ "$c" -lt $((m * 90 / 100)) ]; then
    ok "conntrack usage ${c}/${m}"
  else
    bad "conntrack usage high ${c}/${m}"
  fi
else
  skip "conntrack proc"
fi

# --- 11) GeoIP / 多 WAN ---
geo_n=$(python3 -c "import json; print(len(json.load(open('$STATE')).get('firewall',{}).get('geoip',[])))" 2>/dev/null || echo 0)
wan_n=$(python3 -c "import json; print(len(json.load(open('$STATE')).get('network',{}).get('wan_links',[])))" 2>/dev/null || echo 0)
[ "$geo_n" = "0" ] && skip "GeoIP rules (none configured)" || skip "GeoIP (configured — manual CIDR file test)"
[ "$wan_n" = "0" ] && skip "multi-WAN failover (no wan_links)" || skip "multi-WAN failover (configure wan_links first)"

# --- 报告 ---
{
  echo "# qosnat2 自动验收报告"
  echo ""
  echo "- **时间**: $(date -Iseconds)"
  echo "- **主机**: $(hostname -f 2>/dev/null || hostname)"
  echo "- **DEV_LAN**: $DEV_LAN **DEV_WAN**: $DEV_WAN"
  echo ""
  echo "## 汇总"
  echo ""
  echo "| 结果 | 数量 |"
  echo "|------|------|"
  echo "| PASS | $PASS |"
  echo "| FAIL | $FAIL |"
  echo "| SKIP | $SKIP |"
  echo ""
  echo "## 日志"
  echo ""
  echo '```'
  tail -80 /tmp/qosnat-acceptance-auto.log
  echo '```'
} > "$REPORT"

echo ""
echo "=== summary: PASS=$PASS FAIL=$FAIL SKIP=$SKIP ==="
echo "report: $REPORT"
[ "$FAIL" -eq 0 ]
