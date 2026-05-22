#!/usr/bin/env bash
# P2 iperf 验收：网段 profile 上下行 + /32 host_exact 与 down_bps/up_bps 对账
# 需：本机 iperf3、SSH 免密到内网客户端、qosnatd + eBPF 已加载
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
ENV_FILE="${ENV_FILE:-/etc/qosnat2/env}"
STATE="${STATE_FILE:-/var/lib/qosnat2/state.json}"
BASE="${BASE:-http://127.0.0.1:8080}"
REPORT="${REPORT:-$ROOT/docs/验收报告-p2-iperf.md}"

PASS_N=0
FAIL_N=0
SKIP_N=0
COOKIE=""
TMP=""

ok() { echo "OK   $*"; PASS_N=$((PASS_N + 1)); }
bad() { echo "FAIL $*"; FAIL_N=$((FAIL_N + 1)); }
skip() { echo "SKIP $*"; SKIP_N=$((SKIP_N + 1)); }

# shellcheck source=/dev/null
[ -f "$ENV_FILE" ] && source "$ENV_FILE" 2>/dev/null || true

USER="${ADMIN_USER:-admin}"
PASS_AUTH="${ADMIN_PASS:-${QOSNAT_PASS:-}}"
DEV_LAN="${DEV_LAN:-ens19}"
DEV_WAN="${DEV_WAN:-ens18}"
TEST_CLIENT="${TEST_CLIENT:-100.64.0.254}"
SEG_CIDR="${SEG_CIDR:-100.64.0.0/24}"
TEST_VIP_IP="${TEST_VIP_IP:-100.64.0.254}"
VIP_RATE="${VIP_RATE:-20mbit}"
IPERF_TIME="${IPERF_TIME:-10}"
TOL_PCT="${TOL_PCT:-15}"
IPERF_PORT="${IPERF_PORT:-5201}"

mkdir -p "$(dirname "$REPORT")"
exec > >(tee "/tmp/qosnat-acceptance-p2-iperf.log") 2>&1

echo "=== qosnat2 acceptance-p2-iperf $(date -Iseconds) ==="

TMP=$(mktemp -d)
trap 'cleanup' EXIT
cleanup() {
  pkill -f "iperf3 -s -B ${TEST_IPERF_DST:-}" 2>/dev/null || true
  if [ -n "${COOKIE:-}" ] && [ -f "$COOKIE" ] && [ "${VIP_CLEANUP:-1}" = "1" ]; then
    curl -sf -b "$COOKIE" -X DELETE \
      "$BASE/api/v1/shaper/profiles?cidr=$(python3 -c "import urllib.parse; print(urllib.parse.quote('${TEST_VIP_IP}/32'))")" \
      >/dev/null 2>&1 || true
  fi
  rm -rf "$TMP"
}

[[ -f "$STATE" ]] && grep -q '"setup_complete": true' "$STATE" || { skip "setup 未完成"; exit 0; }
command -v iperf3 >/dev/null || { skip "需要 iperf3"; exit 0; }
command -v python3 >/dev/null || { skip "需要 python3"; exit 0; }

TEST_IPERF_DST="${TEST_IPERF_DST:-}"
if [ -z "$TEST_IPERF_DST" ]; then
  TEST_IPERF_DST=$(ip -4 -o addr show dev "$DEV_WAN" 2>/dev/null | awk '{print $4}' | cut -d/ -f1 | head -1)
fi
[ -n "$TEST_IPERF_DST" ] || { skip "无法解析 WAN IP"; exit 0; }

if ! ssh -o BatchMode=yes -o ConnectTimeout=5 "root@${TEST_CLIENT}" true 2>/dev/null; then
  skip "SSH root@${TEST_CLIENT} 不可用"
  exit 0
fi

# API 登录
if [ -z "$PASS_AUTH" ] && [ -z "${QOSNAT_API_KEY:-}" ]; then
  skip "无 ADMIN_PASS / QOSNAT_API_KEY"
  exit 0
fi
COOKIE=$(mktemp)
api_get() {
  local extra=()
  [ -n "${QOSNAT_API_KEY:-}" ] && extra=(-H "X-API-Key: $QOSNAT_API_KEY")
  curl -sf "${extra[@]}" -b "$COOKIE" "$BASE$1"
}
api_json() {
  local method="$1" path="$2" data="${3:-}"
  local extra=()
  [ -n "${QOSNAT_API_KEY:-}" ] && extra=(-H "X-API-Key: $QOSNAT_API_KEY")
  curl -sf "${extra[@]}" -b "$COOKIE" -X "$method" -H 'Content-Type: application/json' -d "$data" "$BASE$path"
}
if [ -n "$PASS_AUTH" ]; then
  CODE=$(curl -s -o /dev/null -w "%{http_code}" -b "$COOKIE" -c "$COOKIE" \
    -H 'Content-Type: application/json' -d "{\"user\":\"$USER\",\"pass\":\"$PASS_AUTH\"}" "$BASE/api/v1/login")
  [[ "$CODE" == "200" ]] || { bad "API login $CODE"; exit 1; }
  ok "API login"
fi

# down_bps → 期望 Mbit/s（与 store.MbitToBPS 一致：mbit×125000）
bps_to_mbit() {
  python3 -c "print(round($1/125000.0, 2))"
}

in_range() {
  local val="$1" expect="$2"
  python3 - "$val" "$expect" "$TOL_PCT" <<'PY'
import sys
v, e, tol = float(sys.argv[1]), float(sys.argv[2]), float(sys.argv[3])
lo, hi = e * (1 - tol/100), e * (1 + tol/100)
sys.exit(0 if lo <= v <= hi else 1)
PY
}

parse_upload_mbps() {
  ssh -o BatchMode=yes "root@${TEST_CLIENT}" \
    "iperf3 -c ${TEST_IPERF_DST} -p ${IPERF_PORT} -t ${IPERF_TIME} -f m 2>/dev/null" \
    | awk '/Mbits\/sec/ && /sender/ {for(i=1;i<=NF;i++) if($i=="Mbits/sec"){print $(i-1); exit}}'
}

parse_download_mbps() {
  # -R：客户端为 sender，网关 LAN egress 下行整形
  ssh -o BatchMode=yes "root@${TEST_CLIENT}" \
    "iperf3 -c ${TEST_IPERF_DST} -p ${IPERF_PORT} -t ${IPERF_TIME} -f m -R 2>/dev/null" \
    | awk '/Mbits\/sec/ && /sender/ {for(i=1;i<=NF;i++) if($i=="Mbits/sec"){print $(i-1); exit}}'
}

profile_bps() {
  local cidr="$1"
  api_get "/api/v1/shaper/profiles" >"$TMP/profiles.json"
  python3 - "$cidr" "$TMP/profiles.json" <<'PY'
import json, sys
cidr, path = sys.argv[1], sys.argv[2]
d = json.load(open(path))
for p in d.get("profiles") or []:
    if p.get("cidr") == cidr:
        print(p.get("down_bps", 0), p.get("up_bps", 0))
        sys.exit(0)
sys.exit(1)
PY
}

check_rate() {
  local label="$1" measured="$2" expect_mbit="$3"
  if [ -z "$measured" ]; then
    bad "$label parse failed"
    return 1
  fi
  if in_range "$measured" "$expect_mbit"; then
    ok "$label ${measured} Mbit/s (expect ${expect_mbit} ±${TOL_PCT}%)"
    return 0
  fi
  bad "$label ${measured} Mbit/s (expect ${expect_mbit} ±${TOL_PCT}%)"
  return 1
}

# --- 1) 网段 profile 对账（100.64.0.0/24 或 SEG_CIDR）---
if read -r down_bps up_bps < <(profile_bps "$SEG_CIDR" 2>/dev/null); then
  exp_up=$(bps_to_mbit "$up_bps")
  exp_down=$(bps_to_mbit "$down_bps")
  ok "profile $SEG_CIDR API down_bps=$down_bps up_bps=$up_bps → expect down≈${exp_down} up≈${exp_up} Mbit/s"
else
  bad "profile $SEG_CIDR 不在 API 列表"
  exp_up=8
  exp_down=8
fi

pkill -f "iperf3 -s -B ${TEST_IPERF_DST}" 2>/dev/null || true
iperf3 -s -B "$TEST_IPERF_DST" -p "$IPERF_PORT" -D >/dev/null 2>&1 || true
sleep 2

up_m=$(parse_upload_mbps || true)
check_rate "segment upload (ifb/上行)" "$up_m" "$exp_up"

down_m=$(parse_download_mbps || true)
check_rate "segment download -R (${DEV_LAN} egress)" "$down_m" "$exp_down"

if tc filter show dev "$DEV_LAN" parent 1: 2>/dev/null | grep -q classify_egress; then
  ok "${DEV_LAN} egress BPF (下行分类)"
else
  bad "${DEV_LAN} egress 无 classify_egress BPF"
fi

# --- 2) /32 profile + host_exact 对账 ---
VIP_CLEANUP=1
WIZ=$(python3 -c "import json; print(json.dumps({'cidr':'${TEST_VIP_IP}/32','down':'${VIP_RATE}','up':'${VIP_RATE}','mask':32}))")
if api_json POST /api/v1/shaper/wizard "$WIZ" >"$TMP/wiz.json" 2>/dev/null; then
  ok "wizard ${TEST_VIP_IP}/32 ${VIP_RATE}"
else
  bad "wizard POST ${TEST_VIP_IP}/32"
fi
sleep 2

if read -r vdown vup < <(profile_bps "${TEST_VIP_IP}/32"); then
  exp_v=$(bps_to_mbit "$vdown")
  ok "/32 profile down_bps=$vdown → expect ≈${exp_v} Mbit/s"
else
  bad "/32 ${TEST_VIP_IP} 未出现在 profiles"
  exp_v=$(bps_to_mbit "$(python3 -c "r='${VIP_RATE}'.lower(); s='mbit'; n=float(r.replace(s,'').strip() or 0); print(int(n*125000))")")
fi

if command -v bpftool >/dev/null && [ -f /sys/fs/bpf/qosnat2/host_exact ]; then
  bpftool map dump pinned /sys/fs/bpf/qosnat2/host_exact 2>/dev/null >"$TMP/host.json" || true
  bpftool map dump pinned /sys/fs/bpf/qosnat2/profile_lpm 2>/dev/null >"$TMP/lpm.json" || true
  BPF_LINES=$(python3 - "$TEST_VIP_IP" "$vdown" "$TMP/host.json" "$TMP/lpm.json" <<'PY' || true
import json, struct, sys
ip, want_down, host_path, lpm_path = sys.argv[1], int(sys.argv[2]), sys.argv[3], sys.argv[4]
parts = [int(x) for x in ip.split(".")]
host_key = struct.unpack("<I", bytes(parts))[0]
ok_host = ok_lpm = False
try:
    for e in json.load(open(host_path)):
        if e.get("key") == host_key and e.get("value", {}).get("down_bps") == want_down:
            ok_host = True
except (FileNotFoundError, json.JSONDecodeError):
    pass
try:
    for e in json.load(open(lpm_path)):
        k = e.get("key") or {}
        if k.get("prefixlen") == 32 and k.get("addr") == host_key:
            if e.get("value", {}).get("down_bps") == want_down:
                ok_lpm = True
except (FileNotFoundError, json.JSONDecodeError):
    pass
if ok_host:
    print("host")
if ok_lpm:
    print("lpm")
PY
)
  if echo "$BPF_LINES" | grep -q host; then
    ok "host_exact map ${TEST_VIP_IP} down_bps=${vdown}"
  fi
  if echo "$BPF_LINES" | grep -q lpm; then
    ok "profile_lpm /32 ${TEST_VIP_IP} down_bps=${vdown}"
  fi
  if ! echo "$BPF_LINES" | grep -qE 'host|lpm'; then
    bad "BPF map 无 /32 ${TEST_VIP_IP} down_bps=${vdown}"
  fi
else
  skip "bpftool/host_exact 跳过"
fi

sleep 1
up32=$(parse_upload_mbps || true)
check_rate "/32 upload" "$up32" "$exp_v"

down32=$(parse_download_mbps || true)
check_rate "/32 download -R" "$down32" "$exp_v"

pkill -f "iperf3 -s -B ${TEST_IPERF_DST}" 2>/dev/null || true

# --- 报告 ---
{
  echo "# P2 iperf 验收报告"
  echo ""
  echo "- **时间**: $(date -Iseconds)"
  echo "- **网关 WAN**: \`${TEST_IPERF_DST}\` · **LAN**: \`${DEV_LAN}\`"
  echo "- **客户端**: \`${TEST_CLIENT}\` · **网段**: \`${SEG_CIDR}\` · **/32**: \`${TEST_VIP_IP}\` @ \`${VIP_RATE}\`"
  echo "- **容差**: ±${TOL_PCT}% · **时长**: ${IPERF_TIME}s"
  echo ""
  echo "| 项 | 结果 |"
  echo "|----|------|"
  echo "| PASS | ${PASS_N} |"
  echo "| FAIL | ${FAIL_N} |"
  echo "| SKIP | ${SKIP_N} |"
  echo ""
  echo "\`\`\`"
  tail -40 /tmp/qosnat-acceptance-p2-iperf.log 2>/dev/null || true
  echo "\`\`\`"
} >"$REPORT"

echo "=== 报告: $REPORT ==="
if [ "$FAIL_N" -gt 0 ]; then
  exit 1
fi
exit 0
