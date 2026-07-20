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

# staging+rename，避免覆盖正在运行的可执行文件（ETXTBSY / text file busy）
install_atomic() {
  local src="$1" dst="$2"
  local staging="${dst}.new" old="${dst}.old"
  install -m 0755 "${src}" "${staging}"
  if [[ -e "${dst}" ]]; then
    rm -f "${old}"
    mv -f "${dst}" "${old}"
  fi
  mv -f "${staging}" "${dst}"
  rm -f "${old}"
}

[[ -n "${SOURCE}" && -f "${SOURCE}" ]] || die "usage: $0 /path/to/dnsmasq"
if [[ "${EUID:-$(id -u)}" -ne 0 ]]; then
  die "run as root"
fi

if ! "${SOURCE}" --help 2>&1 | grep -q chnroutes-file; then
  die "binary lacks chnroutes-file support: ${SOURCE}"
fi

mkdir -p "${QOSNAT_LIB}"
install_atomic "${SOURCE}" "${QOSNAT_LIB}/dnsmasq-chnroutes"

was_active=0
if command -v systemctl &>/dev/null && systemctl is-active --quiet dnsmasq 2>/dev/null; then
  was_active=1
  systemctl stop dnsmasq || true
fi

if [[ -x "${INSTALL_BIN}" && ! -f "${INSTALL_BIN}.dist" ]]; then
  log "backup ${INSTALL_BIN} -> ${INSTALL_BIN}.dist"
  cp -a "${INSTALL_BIN}" "${INSTALL_BIN}.dist"
fi
install_atomic "${SOURCE}" "${INSTALL_BIN}"

if [[ "${was_active}" -eq 1 ]]; then
  systemctl start dnsmasq || true
elif command -v systemctl &>/dev/null && systemctl is-active --quiet dnsmasq 2>/dev/null; then
  systemctl restart dnsmasq || true
fi

log "installed patched dnsmasq from prebuilt:"
"${INSTALL_BIN}" --version | head -1
