#!/usr/bin/env bash
# 生产网关限制 unattended-upgrades 自动升级（默认 lockdown：完全禁止自动 apt 活动）
#
# 用法:
#   sudo ./scripts/configure-gateway-apt.sh [lockdown|security-only|off]
#   QOSNAT_GATEWAY_APT=lockdown sudo ./deploy-qos-nat.sh start
#
# lockdown       — 禁止自动 update/upgrade（推荐生产 NAT 网关）
# security-only  — 仅安全更新，且黑名单屏蔽 systemd/netplan/frr/内核等
# off            — 移除 qosnat2 apt 配置并恢复 apt 定时器
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
APT_DIR="/etc/apt/apt.conf.d"
MODE="${1:-${QOSNAT_GATEWAY_APT:-lockdown}}"

PERIODIC_LOCKDOWN="${ROOT}/config/apt/20qosnat2-gateway-periodic.conf"
PERIODIC_SECURITY="${ROOT}/config/apt/20qosnat2-gateway-periodic-security.conf"
BLACKLIST="${ROOT}/config/apt/51qosnat2-gateway-unattended.conf"

TARGET_PERIODIC="${APT_DIR}/20qosnat2-gateway.conf"
TARGET_BLACKLIST="${APT_DIR}/51qosnat2-gateway-unattended.conf"

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "run as root"

install_file() {
  local src="$1" dst="$2"
  [[ -f "${src}" ]] || die "missing ${src}"
  install -m 0644 "${src}" "${dst}"
}

remove_qosnat_apt_configs() {
  rm -f "${TARGET_PERIODIC}" "${TARGET_BLACKLIST}"
}

disable_apt_timers() {
  systemctl disable --now apt-daily-upgrade.timer 2>/dev/null || true
  systemctl disable --now apt-daily.timer 2>/dev/null || true
}

enable_apt_timers() {
  systemctl unmask apt-daily-upgrade.timer 2>/dev/null || true
  systemctl unmask apt-daily.timer 2>/dev/null || true
  systemctl enable apt-daily-upgrade.timer 2>/dev/null || true
  systemctl enable apt-daily.timer 2>/dev/null || true
}

case "${MODE}" in
  lockdown)
    install_file "${PERIODIC_LOCKDOWN}" "${TARGET_PERIODIC}"
    install_file "${BLACKLIST}" "${TARGET_BLACKLIST}"
    disable_apt_timers
    log "gateway apt: lockdown — 已禁止 unattended-upgrades 与 apt-daily 定时器"
    log "  维护窗口内手动: apt update && apt upgrade"
    ;;
  security-only)
    install_file "${PERIODIC_SECURITY}" "${TARGET_PERIODIC}"
    install_file "${BLACKLIST}" "${TARGET_BLACKLIST}"
    enable_apt_timers
    log "gateway apt: security-only — 仅安全更新；systemd/netplan/frr/内核等仍在黑名单"
    warn "security-only 仍可能在升级时重启 networkd，生产环境建议 lockdown"
    ;;
  off)
    remove_qosnat_apt_configs
    enable_apt_timers
    log "gateway apt: off — 已移除 qosnat2 apt 配置并恢复 apt 定时器"
    ;;
  *)
    die "unknown mode: ${MODE} (use lockdown|security-only|off)"
    ;;
esac

if command -v systemctl &>/dev/null; then
  systemctl is-enabled apt-daily-upgrade.timer 2>/dev/null | sed 's/^/  apt-daily-upgrade.timer: /' || true
  systemctl is-enabled apt-daily.timer 2>/dev/null | sed 's/^/  apt-daily.timer: /' || true
fi
