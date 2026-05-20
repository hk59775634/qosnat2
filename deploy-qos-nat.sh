#!/usr/bin/env bash
# qosnat2 — 单机双网卡 QoS+NAT 部署（无 netns / flowtable）
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEV_LAN="${DEV_LAN:-}"
DEV_WAN="${DEV_WAN:-}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-QosNat@2026}"
ADMIN_PORT="${ADMIN_PORT:-8080}"
STATE_DIR="${STATE_DIR:-/var/lib/qosnat2}"
CONFIG_DIR="${CONFIG_DIR:-/etc/qosnat2}"
SHARED_IP_1="${SHARED_IP_1:-}"
POLICY_ROUTES="${POLICY_ROUTES:-10.0.0.0/8}"

QOSNATD_SRC="${ROOT}/cmd/qosnatd"
QOSNATD_BIN="/usr/local/bin/qosnatd"
SYSCTL_CONF="/etc/sysctl.d/99-qosnat2.conf"
DEPLOY_ENV="${CONFIG_DIR}/deploy.env"

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

require_root() {
  [[ "$(id -u)" -eq 0 ]] || die "请使用 root 或 sudo 运行"
}

require_nics() {
  [[ -n "${DEV_LAN}" && -n "${DEV_WAN}" ]] || die "必须设置 DEV_LAN 与 DEV_WAN，例如: DEV_LAN=vlan.3003 DEV_WAN=vlan.907 $0 start"
  ip link show "${DEV_LAN}" &>/dev/null || die "DEV_LAN=${DEV_LAN} 不存在"
  ip link show "${DEV_WAN}" &>/dev/null || die "DEV_WAN=${DEV_WAN} 不存在"
  if ip netns list 2>/dev/null | grep -q .; then
    warn "检测到 netns；qosnat2 不使用 netns，请确认 WAN 在宿主机"
  fi
}

save_deploy_env() {
  mkdir -p "${CONFIG_DIR}"
  cat > "${DEPLOY_ENV}" <<EOF
DEV_LAN=${DEV_LAN}
DEV_WAN=${DEV_WAN}
ADMIN_PORT=${ADMIN_PORT}
EOF
}

install_deps() {
  if command -v apt-get &>/dev/null; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
      iproute2 nftables golang-go clang llvm libbpf-dev make \
      wireguard-tools tcpdump conntrack \
      || warn "apt 安装部分失败，请手动安装依赖"
  fi
}

build_bpf() {
  if command -v clang &>/dev/null; then
    log "编译 classify.bpf.o..."
    (cd "${ROOT}/bpf" && make && make install INSTALL_DIR=/usr/lib/qosnat2) || warn "BPF 编译失败，P1 Map 需 clang/libbpf"
  else
    warn "未找到 clang，跳过 BPF 编译"
  fi
}

build_web() {
  if command -v npm &>/dev/null && [[ -f "${ROOT}/web/package.json" ]]; then
    log "构建 Web UI (Vue3)..."
    (cd "${ROOT}/web" && npm ci --silent 2>/dev/null || npm install --silent) && npm run build) || warn "Web 构建失败，将使用旧静态页"
  else
    warn "未找到 npm，跳过 web/dist 构建"
  fi
}

build_qosnatd() {
  log "编译 qosnatd..."
  (cd "${ROOT}" && go mod tidy && go build -o "${ROOT}/bin/qosnatd" ./cmd/qosnatd)
  local real
  real="$(readlink -f "${ROOT}/bin/qosnatd")"
  install -m 0755 "${real}" "${QOSNATD_BIN}"
  log "已安装 ${QOSNATD_BIN} <- ${real}"
}

write_env_file() {
  mkdir -p "${CONFIG_DIR}"
  cat > "${CONFIG_DIR}/env" <<EOF
ADMIN_USER=${ADMIN_USER}
ADMIN_PASS=${ADMIN_PASS}
ADMIN_PORT=${ADMIN_PORT}
DEV_LAN=${DEV_LAN}
DEV_WAN=${DEV_WAN}
STATE_FILE=${STATE_DIR}/state.json
SESSION_FILE=${STATE_DIR}/sessions.json
OPENAPI_PATH=${ROOT}/api/openapi.yaml
WEB_ROOT=${ROOT}/web/dist
# 若未构建 dist，回退 web/
[[ -f "${WEB_ROOT}/index.html" ]] || WEB_ROOT=${ROOT}/web
EOF
  chmod 0600 "${CONFIG_DIR}/env"
}

init_state() {
  mkdir -p "${STATE_DIR}"
  local sf="${STATE_DIR}/state.json"
  if [[ -f "${sf}" ]]; then
    return 0
  fi
  local shared_json="[]"
  if [[ -n "${SHARED_IP_1}" ]]; then
    shared_json="[\"${SHARED_IP_1}\"]"
  fi
  local routes_json
  routes_json="$(printf '%s' "${POLICY_ROUTES}" | sed 's/,/","/g; s/^/["/; s/$/"]/')"
  cat > "${sf}" <<EOF
{
  "policy_routes": ${routes_json},
  "shared_ips": ${shared_json},
  "static_mappings": {},
  "prefix_mappings": {},
  "shaper": {
    "policy_cidr": "10.0.0.0/8",
    "default_profile": { "down": "8mbit", "up": "8mbit", "host_mask": 32 },
    "profiles": [],
    "hosts": {},
    "leaf": "fq_codel",
    "idle_timeout_sec": 300
  },
  "firewall": { "wan_port_forwards": [], "rules": [] },
  "system": { "sysctl": {}, "hostname": "qosnat2" },
  "api_keys": []
}
EOF
  chmod 0600 "${sf}"
  log "已创建 ${sf}"
}

setup_sysctl() {
  cat > "${SYSCTL_CONF}" <<'EOF'
net.ipv4.ip_forward = 1
net.ipv4.conf.all.rp_filter = 0
net.core.rmem_max = 134217728
net.core.wmem_max = 134217728
net.netfilter.nf_conntrack_max = 2097152
EOF
  sysctl --system >/dev/null 2>&1 || sysctl -p "${SYSCTL_CONF}" || true
}

setup_tc_placeholder() {
  modprobe ifb sch_htb sch_fq_codel sch_fq cls_bpf act_bpf act_mirred 2>/dev/null || true
  ip link show ifb0 &>/dev/null || ip link add ifb0 type ifb
  ip link set ifb0 up
  tc qdisc del dev "${DEV_LAN}" root 2>/dev/null || true
  tc qdisc add dev "${DEV_LAN}" root handle 1: htb default 1
  tc class add dev "${DEV_LAN}" parent 1: classid 1:1 htb rate 10gbit ceil 10gbit 2>/dev/null || true
  tc qdisc add dev "${DEV_LAN}" parent 1:1 fq_codel 2>/dev/null || true
  tc qdisc del dev ifb0 root 2>/dev/null || true
  tc qdisc add dev ifb0 root handle 1: htb default 1
  tc class add dev ifb0 parent 1: classid 1:1 htb rate 10gbit ceil 10gbit 2>/dev/null || true
  tc qdisc add dev ifb0 parent 1:1 fq_codel 2>/dev/null || true
  tc qdisc del dev "${DEV_LAN}" clsact 2>/dev/null || true
  tc qdisc add dev "${DEV_LAN}" clsact
  mkdir -p /sys/fs/bpf/qosnat2
  log "TC: HTB 根 + clsact 占位（LAN=${DEV_LAN}, ifb0）"
}

install_systemd() {
  cat > /etc/systemd/system/qosnatd.service <<EOF
[Unit]
Description=qosnat2 control plane (REST + nft + tc)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
EnvironmentFile=${CONFIG_DIR}/env
ExecStart=${QOSNATD_BIN}
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF
  cat > /etc/systemd/system/qos-nat.service <<EOF
[Unit]
Description=qosnat2 one-shot apply (tc/sysctl replay)
After=network-online.target

[Service]
Type=oneshot
EnvironmentFile=${CONFIG_DIR}/env
ExecStart=${QOSNATD_BIN} apply-state
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable qos-nat.service qosnatd.service
}

cmd_start() {
  require_root
  [[ -f "${DEPLOY_ENV}" ]] && set -a && source "${DEPLOY_ENV}" && set +a
  require_nics
  install_deps
  build_bpf
  build_web
  build_qosnatd
  write_env_file
  init_state
  setup_sysctl
  setup_tc_placeholder
  save_deploy_env
  install_systemd
  timeout 60 systemctl restart qos-nat.service || warn "qos-nat.service 超时或失败"
  timeout 30 systemctl restart qosnatd.service || systemctl start qosnatd.service || warn "qosnatd 启动失败"
  log "部署完成。健康检查: curl -s http://127.0.0.1:${ADMIN_PORT}/api/v1/health"
  log "若 shared_ips 为空，请 POST /api/v1/nat/shared-ips 后再加载 nft"
}

cmd_stop() {
  require_root
  systemctl stop qosnatd.service 2>/dev/null || true
  systemctl stop qos-nat.service 2>/dev/null || true
  nft flush ruleset 2>/dev/null || true
  [[ -n "${DEV_LAN}" ]] && tc qdisc del dev "${DEV_LAN}" clsact 2>/dev/null || true
  log "已停止服务并 flush nft（TC 根未删除，避免断流）"
}

cmd_status() {
  systemctl status qosnatd.service --no-pager 2>/dev/null || true
  curl -sf "http://127.0.0.1:${ADMIN_PORT}/api/v1/health" 2>/dev/null | head -c 500 || warn "health 不可达"
  echo
  nft list ruleset 2>/dev/null | head -40 || true
}

usage() {
  cat <<EOF
用法: DEV_LAN=... DEV_WAN=... [SHARED_IP_1=公网IP] $0 {start|stop|status}

  start  — 编译安装 qosnatd、sysctl、ifb/HTB/clsact、systemd
  stop   — 停止服务
  status — 服务与健康检查

禁止: netns、flowtable、WAN 移入 netns
EOF
}

case "${1:-}" in
  start)  cmd_start ;;
  stop)   cmd_stop ;;
  status) cmd_status ;;
  *)      usage; exit 1 ;;
esac
