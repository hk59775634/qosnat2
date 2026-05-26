#!/usr/bin/env bash
# qosnat2 — 安装后仅启动 Web 控制面；数据面在首次引导完成后生效
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEV_LAN="${DEV_LAN:-}"
DEV_WAN="${DEV_WAN:-}"
ADMIN_USER="${ADMIN_USER:-admin}"
ADMIN_PASS="${ADMIN_PASS:-}"
INITIAL_ADMIN_FILE="${CONFIG_DIR}/initial-admin.txt"
ADMIN_PORT="${ADMIN_PORT:-8080}"
STATE_DIR="${STATE_DIR:-/var/lib/qosnat2}"
CONFIG_DIR="${CONFIG_DIR:-/etc/qosnat2}"
SKIP_WEB_BUILD="${SKIP_WEB_BUILD:-0}"
BUILD_WEB="${BUILD_WEB:-0}"

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
  if [[ "${BUILD_WEB}" != "1" && -f "${WEB_DIST}/index.html" ]]; then
    log "web/dist 已存在，跳过构建（使用 -BuildWeb 强制重建）"
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

gen_admin_password() {
  if [[ -n "${ADMIN_PASS}" ]]; then
    return 0
  fi
  if command -v openssl &>/dev/null; then
    ADMIN_PASS="$(openssl rand -base64 32 | tr -dc 'A-Za-z0-9' | head -c 20)"
  else
    ADMIN_PASS="$(tr -dc 'A-Za-z0-9' </dev/urandom | head -c 20)"
  fi
}

write_initial_admin_notice() {
  mkdir -p "${CONFIG_DIR}"
  cat > "${INITIAL_ADMIN_FILE}" <<EOF
# qosnat2 初始管理员（安装时生成，请妥善保存后删除本文件）
ADMIN_USER=${ADMIN_USER}
ADMIN_PASS=${ADMIN_PASS}
EOF
  chmod 0600 "${INITIAL_ADMIN_FILE}"
}

write_env_file() {
  gen_admin_password
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
  echo "ADMIN_USER=${ADMIN_USER}" >> "${CONFIG_DIR}/env"
  echo "ADMIN_PASS=${ADMIN_PASS}" >> "${CONFIG_DIR}/env"
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
# 控制面需 root 写 TC/nft/sysctl；仅启用不影响数据面的轻量加固
PrivateTmp=yes
ProtectHome=read-only
ProtectKernelModules=yes
ProtectControlGroups=yes
LimitNOFILE=65535

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
  write_initial_admin_notice
  init_state
  save_deploy_env
  install_systemd
  systemctl restart qosnatd.service || systemctl start qosnatd.service || die "qosnatd 启动失败"
  log "安装完成：仅 Web UI 已启动（数据面未加载，直至首次引导完成）"
  log "=========================================="
  log "初始管理员（请先登录，再完成 Web 引导）"
  log "  用户: ${ADMIN_USER}"
  log "  口令: ${ADMIN_PASS}"
  log "  已写入: ${INITIAL_ADMIN_FILE} （权限 0600，用后请删除）"
  log "=========================================="
  log "打开浏览器: http://$(hostname -I 2>/dev/null | awk '{print $1}'):${ADMIN_PORT}/#/login"
  log "登录后进入「初始设置」向导"
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
用法: $0 [选项] {start|stop|status}

  start  — 编译安装 qosnatd + Web UI，仅启动控制面（不加载 NAT/QoS）
  stop   — 停止服务
  status — 服务与健康检查

选项:
  -BuildWeb     强制 npm run build（即使 web/dist 已存在）
  -SkipWeb      跳过前端构建（等同 SKIP_WEB_BUILD=1）

首次安装会生成随机管理员口令（可用 ADMIN_PASS= 覆盖）；请先登录再完成「初始设置」向导。
数据面在向导点击「完成」后才会 apply（并启用 qos-nat.service）。

可选环境变量:
  DEV_LAN=... DEV_WAN=... ADMIN_USER=admin ADMIN_PASS=...（不设则随机 20 位）
  ADMIN_PORT=8080  SKIP_WEB_BUILD=1  BUILD_WEB=1
EOF
}

CMD=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -BuildWeb) BUILD_WEB=1; shift ;;
    -SkipWeb)  SKIP_WEB_BUILD=1; shift ;;
    start|stop|status) CMD="$1"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "未知参数: $1（$0 -h 查看帮助）" ;;
  esac
done

case "${CMD:-}" in
  start)  cmd_start ;;
  stop)   cmd_stop ;;
  status) cmd_status ;;
  *)      usage; exit 1 ;;
esac
