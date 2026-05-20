#!/usr/bin/env bash
# qosnat2 — 安装后仅启动 Web 控制面；数据面在首次引导完成后生效
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEV_LAN="${DEV_LAN:-}"
DEV_WAN="${DEV_WAN:-}"
ADMIN_USER="${ADMIN_USER:-}"
ADMIN_PASS="${ADMIN_PASS:-}"
ADMIN_PORT="${ADMIN_PORT:-8080}"
STATE_DIR="${STATE_DIR:-/var/lib/qosnat2}"
CONFIG_DIR="${CONFIG_DIR:-/etc/qosnat2}"
SKIP_WEB_BUILD="${SKIP_WEB_BUILD:-0}"

QOSNATD_SRC="${ROOT}/cmd/qosnatd"
QOSNATD_BIN="/usr/local/bin/qosnatd"
SYSCTL_CONF="/etc/sysctl.d/99-qosnat2.conf"
DEPLOY_ENV="${CONFIG_DIR}/deploy.env"
WEB_DIST="${ROOT}/web/dist"

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

require_root() {
  [[ "$(id -u)" -eq 0 ]] || die "请使用 root 或 sudo 运行"
}

save_deploy_env() {
  mkdir -p "${CONFIG_DIR}"
  cat > "${DEPLOY_ENV}" <<EOF
# qosnat2 deploy metadata
ADMIN_PORT=${ADMIN_PORT}
STATE_DIR=${STATE_DIR}
EOF
  if [[ -n "${DEV_LAN}" ]]; then
    echo "DEV_LAN=${DEV_LAN}" >> "${DEPLOY_ENV}"
  fi
  if [[ -n "${DEV_WAN}" ]]; then
    echo "DEV_WAN=${DEV_WAN}" >> "${DEPLOY_ENV}"
  fi
}

install_deps() {
  if command -v apt-get &>/dev/null; then
    DEBIAN_FRONTEND=noninteractive apt-get update -qq
    DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
      iproute2 nftables golang-go clang llvm libbpf-dev make \
      wireguard-tools tcpdump conntrack dnsmasq \
      || warn "apt 安装部分失败，请手动安装依赖"
  fi
}

build_bpf() {
  if command -v clang &>/dev/null; then
    log "编译 classify.bpf.o..."
    (cd "${ROOT}/bpf" && make && make install INSTALL_DIR=/usr/lib/qosnat2) || warn "BPF 编译失败，引导完成后可在 UI 重载 eBPF"
  else
    warn "未找到 clang，跳过 BPF 编译"
  fi
}

build_web() {
  if [[ "${SKIP_WEB_BUILD}" == "1" ]]; then
    warn "SKIP_WEB_BUILD=1，跳过前端构建"
    return 0
  fi
  if command -v npm &>/dev/null && [[ -f "${ROOT}/web/package.json" ]]; then
    log "构建 Web UI (Vue3)..."
    (cd "${ROOT}/web" && (npm ci --silent 2>/dev/null || npm install --silent) && npm run build) || warn "Web 构建失败"
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

resolve_web_root() {
  if [[ -f "${WEB_DIST}/index.html" ]]; then
    echo "${WEB_DIST}"
  else
    echo "${ROOT}/web"
  fi
}

write_env_file() {
  local web_root
  web_root="$(resolve_web_root)"
  mkdir -p "${CONFIG_DIR}"
  cat > "${CONFIG_DIR}/env" <<EOF
# qosnat2 — 首次访问 Web UI 完成引导前，数据面不会加载
ADMIN_PORT=${ADMIN_PORT}
STATE_FILE=${STATE_DIR}/state.json
SESSION_FILE=${STATE_DIR}/sessions.json
OPENAPI_PATH=${ROOT}/api/openapi.yaml
WEB_ROOT=${web_root}
EOF
  if [[ -n "${ADMIN_USER}" ]]; then
    echo "ADMIN_USER=${ADMIN_USER}" >> "${CONFIG_DIR}/env"
  fi
  if [[ -n "${ADMIN_PASS}" ]]; then
    echo "ADMIN_PASS=${ADMIN_PASS}" >> "${CONFIG_DIR}/env"
  fi
  if [[ -n "${DEV_LAN}" ]]; then
    echo "DEV_LAN=${DEV_LAN}" >> "${CONFIG_DIR}/env"
  fi
  if [[ -n "${DEV_WAN}" ]]; then
    echo "DEV_WAN=${DEV_WAN}" >> "${CONFIG_DIR}/env"
  fi
  chmod 0600 "${CONFIG_DIR}/env"
}

init_state() {
  mkdir -p "${STATE_DIR}"
  local sf="${STATE_DIR}/state.json"
  if [[ -f "${sf}" ]]; then
    log "保留已有 ${sf}"
    return 0
  fi
  cat > "${sf}" <<'EOF'
{
  "setup_complete": false,
  "policy_routes": ["10.0.0.0/8"],
  "shared_ips": [],
  "static_mappings": {},
  "prefix_mappings": {},
  "shaper": {
    "policy_cidr": "10.0.0.0/8",
    "default_profile": { "down": "8mbit", "up": "8mbit", "host_mask": 32 },
    "profiles": [],
    "leaf": "fq_codel",
    "idle_timeout_sec": 300
  },
  "firewall": { "wan_port_forwards": [] },
  "system": { "sysctl": {}, "hostname": "qosnat2" },
  "dhcp": {
    "enabled": false,
    "interface": "",
    "range_start": "192.168.1.100",
    "range_end": "192.168.1.254",
    "router": "192.168.1.1",
    "netmask": "255.255.255.0",
    "dns_servers": ["8.8.8.8", "1.1.1.1"],
    "lease_time_sec": 86400,
    "authoritative": true,
    "static_leases": []
  },
  "api_keys": []
}
EOF
  chmod 0600 "${sf}"
  log "已创建 ${sf}（setup_complete=false，等待 Web 引导）"
}

install_systemd() {
  cat > /etc/systemd/system/qosnatd.service <<EOF
[Unit]
Description=qosnat2 Web control plane (REST + static UI)
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
Description=qosnat2 dataplane apply (enabled after setup wizard)
After=network-online.target qosnatd.service

[Service]
Type=oneshot
EnvironmentFile=${CONFIG_DIR}/env
ExecStart=${QOSNATD_BIN} apply-state
RemainAfterExit=yes

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable qosnatd.service
  systemctl disable qos-nat.service 2>/dev/null || true
}

cmd_start() {
  require_root
  [[ -f "${DEPLOY_ENV}" ]] && set -a && source "${DEPLOY_ENV}" && set +a
  install_deps
  build_bpf
  build_web
  build_qosnatd
  write_env_file
  init_state
  save_deploy_env
  install_systemd
  systemctl restart qosnatd.service || systemctl start qosnatd.service || die "qosnatd 启动失败"
  log "安装完成：仅 Web UI 已启动（数据面未加载，直至首次引导完成）"
  log "打开浏览器: http://$(hostname -I 2>/dev/null | awk '{print $1}'):${ADMIN_PORT}/"
  log "将自动进入「初始设置」向导（类似 AdGuard Home）"
  curl -sf "http://127.0.0.1:${ADMIN_PORT}/api/v1/health" 2>/dev/null | head -c 200 || warn "health 暂不可达"
  echo
}

cmd_stop() {
  require_root
  systemctl stop qosnatd.service 2>/dev/null || true
  systemctl stop qos-nat.service 2>/dev/null || true
  nft flush ruleset 2>/dev/null || true
  log "已停止 qosnatd / qos-nat"
}

cmd_status() {
  systemctl status qosnatd.service --no-pager 2>/dev/null || true
  curl -sf "http://127.0.0.1:${ADMIN_PORT}/api/v1/health" 2>/dev/null || warn "health 不可达"
  echo
  curl -sf "http://127.0.0.1:${ADMIN_PORT}/api/v1/setup/status" 2>/dev/null || true
  echo
}

usage() {
  cat <<EOF
用法: $0 {start|stop|status}

  start  — 编译安装 qosnatd + Web UI，仅启动控制面（不加载 NAT/QoS）
  stop   — 停止服务
  status — 服务与健康检查

首次安装后请用浏览器完成「初始设置」向导（管理员账号、LAN/WAN 网卡等）。
数据面在向导点击「完成」后才会 apply（并启用 qos-nat.service）。

可选环境变量（高级/自动化，跳过向导部分步骤）:
  DEV_LAN=... DEV_WAN=... ADMIN_USER=... ADMIN_PASS=...
  ADMIN_PORT=8080  SKIP_WEB_BUILD=1
EOF
}

case "${1:-}" in
  start)  cmd_start ;;
  stop)   cmd_stop ;;
  status) cmd_status ;;
  *)      usage; exit 1 ;;
esac
