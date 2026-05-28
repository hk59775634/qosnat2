#!/usr/bin/env bash
# qosnat2 一键卸载 / 删除
#
# 本地仓库：
#   sudo bash /opt/qosnat2/scripts/uninstall.sh
#   sudo /opt/qosnat2/deploy-qos-nat.sh uninstall
#
# 远程（未克隆仓库时，需已有 /opt/qosnat2 或仅清理已安装组件）：
#   curl -fsSL -H 'Cache-Control: no-cache' \
#     "https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/uninstall.sh?t=$(date +%s)" | bash
#
# 环境变量：
#   QOSNAT_INSTALL_DIR=/opt/qosnat2   源码目录（--purge-repo 时删除）
#   QOSNAT_CONFIG_DIR=/etc/qosnat2
#   QOSNAT_STATE_DIR=/var/lib/qosnat2
#   QOSNAT_YES=1                      等同 -y，跳过确认
#   QOSNAT_KEEP_DATA=1                保留配置与 state（仅停服务并清理数据面）
#   QOSNAT_PURGE_REPO=1               删除源码目录
#   QOSNAT_SKIP_DATAPLANE=1           不清理 tc/nft/bpf（仅停服务与删文件）

set -euo pipefail

QOSNAT_INSTALL_RAW_URL="${QOSNAT_INSTALL_RAW_URL:-https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/uninstall.sh}"
QOSNAT_INSTALL_DIR="${QOSNAT_INSTALL_DIR:-/opt/qosnat2}"
CONFIG_DIR="${QOSNAT_CONFIG_DIR:-/etc/qosnat2}"
STATE_DIR="${QOSNAT_STATE_DIR:-/var/lib/qosnat2}"
QOSNATD_BIN="/usr/local/bin/qosnatd"
SYSCTL_CONF="/etc/sysctl.d/99-qosnat2.conf"
BPF_PIN_DIR="/sys/fs/bpf/qosnat2"
BPF_OBJ="/usr/lib/qosnat2/classify.bpf.o"
NFT_RULES="/etc/qosnat2/nftables-qosnat.nft"
DNSMASQ_DROPIN="/etc/dnsmasq.d/qosnat2.conf"

KEEP_DATA=0
PURGE_REPO=0
SKIP_DATAPLANE=0
ASSUME_YES=0

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

should_refresh_uninstall_script() {
  [[ "${QOSNAT_SKIP_SCRIPT_REFRESH:-0}" == "1" ]] && return 1
  [[ -n "${QOSNAT_INSTALL_REFRESHED:-}" ]] && return 1
  local src="${BASH_SOURCE[0]:-}"
  [[ -f "${src}" ]] && [[ "${src}" == */uninstall.sh ]] && return 1
  return 0
}

bootstrap_refresh_uninstall_script() {
  should_refresh_uninstall_script || return 0
  command -v curl &>/dev/null || return 0
  local tmp
  tmp="$(mktemp /tmp/qosnat2-uninstall.XXXXXX.sh)"
  if ! curl -fsSL -H 'Cache-Control: no-cache' -H 'Pragma: no-cache' \
      "${QOSNAT_INSTALL_RAW_URL}?t=$(date +%s)" -o "${tmp}" 2>/dev/null; then
    rm -f "${tmp}"
    warn "无法从 GitHub 拉取最新 uninstall.sh，将使用当前脚本继续"
    return 0
  fi
  chmod 700 "${tmp}"
  log "已拉取最新 uninstall.sh，继续卸载…"
  export QOSNAT_INSTALL_REFRESHED=1
  exec env QOSNAT_INSTALL_REFRESHED=1 bash "${tmp}" "$@"
}

usage() {
  cat <<EOF
用法: $0 [选项]

一键停止 qosnat2 服务并清理数据面（tc / nft / bpf），默认删除配置与状态目录。

选项:
  -y, --yes           不询问确认，直接执行
  --keep-data         保留 ${CONFIG_DIR} 与 ${STATE_DIR}
  --purge-repo        同时删除源码目录 ${QOSNAT_INSTALL_DIR}
  --skip-dataplane    仅停服务并删安装文件，不清理 tc/nft/bpf
  -h, --help          显示帮助

说明:
  - nft 将尝试删除 table inet qosnat；若不存在则执行 flush ruleset（与 deploy stop 一致）。
  - 不会卸载 apt 依赖（Go、clang、nodejs 等）；不会删除通过 install-ocserv.sh 安装的 ocserv。
  - 若曾启用 WireGuard/DHCP，请卸载后自行检查 wg0、dnsmasq 是否仍需运行。
EOF
}

require_root() {
  [[ "$(id -u)" -eq 0 ]] || die "请使用 root 或 sudo 运行"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -y|--yes) ASSUME_YES=1; shift ;;
      --keep-data) KEEP_DATA=1; shift ;;
      --purge-repo) PURGE_REPO=1; shift ;;
      --skip-dataplane) SKIP_DATAPLANE=1; shift ;;
      -h|--help) usage; exit 0 ;;
      *) die "未知参数: $1（$0 -h 查看帮助）" ;;
    esac
  done
  if [[ "${QOSNAT_YES:-0}" == "1" ]]; then ASSUME_YES=1; fi
  if [[ "${QOSNAT_KEEP_DATA:-0}" == "1" ]]; then KEEP_DATA=1; fi
  if [[ "${QOSNAT_PURGE_REPO:-0}" == "1" ]]; then PURGE_REPO=1; fi
  if [[ "${QOSNAT_SKIP_DATAPLANE:-0}" == "1" ]]; then SKIP_DATAPLANE=1; fi
}

confirm() {
  [[ "${ASSUME_YES}" == "1" ]] && return 0
  echo ""
  echo "将停止 qosnatd / qos-nat，清理 TC/nft/BPF，并删除 qosnat2 安装文件。"
  [[ "${KEEP_DATA}" == "0" ]] && echo "  - 删除: ${CONFIG_DIR}、${STATE_DIR}"
  [[ "${PURGE_REPO}" == "1" ]] && echo "  - 删除源码: ${QOSNAT_INSTALL_DIR}"
  [[ "${SKIP_DATAPLANE}" == "1" ]] && echo "  - 已指定 --skip-dataplane，不清理数据面"
  echo ""
  read -r -p "确认继续？[y/N] " ans
  case "${ans,,}" in
    y|yes) ;;
    *) die "已取消" ;;
  esac
}

load_env() {
  DEV_LAN=""
  DEV_WAN=""
  if [[ -f "${CONFIG_DIR}/env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "${CONFIG_DIR}/env" 2>/dev/null || true
    set +a
  elif [[ -f "${CONFIG_DIR}/deploy.env" ]]; then
    set -a
    # shellcheck disable=SC1091
    source "${CONFIG_DIR}/deploy.env" 2>/dev/null || true
    set +a
  fi
}

stop_services() {
  log "停止并禁用 systemd 服务…"
  systemctl stop qosnatd.service 2>/dev/null || true
  systemctl stop qos-nat.service 2>/dev/null || true
  systemctl disable qosnatd.service 2>/dev/null || true
  systemctl disable qos-nat.service 2>/dev/null || true
}

teardown_wireguard() {
  if command -v wg-quick &>/dev/null && ip link show wg0 &>/dev/null; then
    log "关闭 WireGuard 接口 wg0（若存在）…"
    wg-quick down wg0 2>/dev/null || true
  fi
}

teardown_dnsmasq() {
  if [[ -f "${DNSMASQ_DROPIN}" ]]; then
    log "移除 dnsmasq 配置 ${DNSMASQ_DROPIN} …"
    rm -f "${DNSMASQ_DROPIN}"
    if command -v systemctl &>/dev/null && systemctl is-active --quiet dnsmasq 2>/dev/null; then
      systemctl reload dnsmasq 2>/dev/null || systemctl restart dnsmasq 2>/dev/null || true
    fi
  fi
}

tc_del_qdisc() {
  local dev="$1"
  [[ -n "${dev}" ]] || return 0
  ip link show "${dev}" &>/dev/null || return 0
  tc qdisc del dev "${dev}" clsact 2>/dev/null || true
  tc qdisc del dev "${dev}" ingress 2>/dev/null || true
  tc qdisc del dev "${dev}" root 2>/dev/null || true
}

teardown_dataplane() {
  [[ "${SKIP_DATAPLANE}" == "1" ]] && {
    warn "已跳过数据面清理（--skip-dataplane）"
    return 0
  }

  load_env
  log "清理 TC 队列（LAN=${DEV_LAN:-—} WAN=${DEV_WAN:-—} ifb0）…"
  tc_del_qdisc "${DEV_LAN:-}"
  tc_del_qdisc "${DEV_WAN:-}"
  tc_del_qdisc ifb0

  log "清理 nftables…"
  if command -v nft &>/dev/null; then
    if nft list table inet qosnat &>/dev/null; then
      nft delete table inet qosnat 2>/dev/null || nft flush ruleset 2>/dev/null || true
    else
      nft flush ruleset 2>/dev/null || true
    fi
  fi

  if [[ -d "${BPF_PIN_DIR}" ]]; then
    log "移除 BPF pin: ${BPF_PIN_DIR}"
    rm -rf "${BPF_PIN_DIR}"
  fi
}

remove_installed_files() {
  log "移除 systemd 单元与二进制…"
  rm -f /etc/systemd/system/qosnatd.service
  rm -f /etc/systemd/system/qos-nat.service
  systemctl daemon-reload 2>/dev/null || true
  systemctl reset-failed qosnatd.service 2>/dev/null || true
  systemctl reset-failed qos-nat.service 2>/dev/null || true

  rm -f "${QOSNATD_BIN}"
  rm -f /etc/qosnat2/release-tag

  if [[ -f "${SYSCTL_CONF}" ]]; then
    log "移除 ${SYSCTL_CONF}"
    rm -f "${SYSCTL_CONF}"
    if command -v sysctl &>/dev/null; then
      sysctl --system &>/dev/null || true
    fi
  fi

  rm -f "${BPF_OBJ}"
  rmdir /usr/lib/qosnat2 2>/dev/null || true
  rm -f "${NFT_RULES}"

  if [[ "${KEEP_DATA}" == "0" ]]; then
    log "删除配置与状态目录…"
    rm -rf "${CONFIG_DIR}" "${STATE_DIR}"
  else
    warn "已保留 ${CONFIG_DIR} 与 ${STATE_DIR}（--keep-data）"
  fi

  if [[ "${PURGE_REPO}" == "1" ]]; then
    if [[ -d "${QOSNAT_INSTALL_DIR}" ]]; then
      log "删除源码目录 ${QOSNAT_INSTALL_DIR} …"
      rm -rf "${QOSNAT_INSTALL_DIR}"
    fi
  elif [[ -d "${QOSNAT_INSTALL_DIR}" ]]; then
    warn "保留源码目录 ${QOSNAT_INSTALL_DIR}（彻底删除请加 --purge-repo）"
  fi
}

main() {
  parse_args "$@"
  require_root
  bootstrap_refresh_uninstall_script "$@"
  confirm
  echo ""
  log "========== qosnat2 卸载开始 =========="
  stop_services
  teardown_wireguard
  teardown_dnsmasq
  teardown_dataplane
  remove_installed_files
  log "========== qosnat2 已卸载 =========="
  log "如需重新安装: curl -fsSL -H 'Cache-Control: no-cache' \".../install.sh?t=\$(date +%s)\" | bash"
}

main "$@"
