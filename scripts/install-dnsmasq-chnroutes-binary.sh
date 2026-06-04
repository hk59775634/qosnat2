#!/usr/bin/env bash
# 将已编译的 chnroutes dnsmasq 安装到 QOSNAT_LIB 并替换 /usr/sbin/dnsmasq（保留 .dist 备份）。
# 用法: install-dnsmasq-chnroutes-binary.sh /path/to/dnsmasq
set -euo pipefail

SOURCE="${1:-}"
INSTALL_BIN="${INSTALL_BIN:-/usr/sbin/dnsmasq}"
QOSNAT_LIB="${QOSNAT_LIB:-/usr/local/lib/qosnat2}"

log() { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die() { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

[[ -n "${SOURCE}" && -f "${SOURCE}" ]] || die "usage: $0 /path/to/dnsmasq"
if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  die "run as root"
fi

if ! "${SOURCE}" --help 2>&1 | grep -q chnroutes-file; then
  die "binary lacks chnroutes-file support: ${SOURCE}"
fi

mkdir -p "${QOSNAT_LIB}"
install -m 0755 "${SOURCE}" "${QOSNAT_LIB}/dnsmasq-chnroutes"

if [[ -x "${INSTALL_BIN}" && ! -f "${INSTALL_BIN}.dist" ]]; then
  log "backup ${INSTALL_BIN} -> ${INSTALL_BIN}.dist"
  cp -a "${INSTALL_BIN}" "${INSTALL_BIN}.dist"
fi
install -m 0755 "${SOURCE}" "${INSTALL_BIN}"

if command -v systemctl &>/dev/null && systemctl is-active --quiet dnsmasq 2>/dev/null; then
  systemctl restart dnsmasq || true
fi

log "installed patched dnsmasq from prebuilt:"
"${INSTALL_BIN}" --version | head -1
