#!/usr/bin/env bash
# 安装 FRR（Ubuntu 24.04），启用 zebra + staticd，并启动服务。
set -euo pipefail

log() { echo "[$(date '+%F %T')] $*"; }
die() { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

[[ "${EUID:-$(id -u)}" -ne 0 ]] && die "run as root"

if ! command -v apt-get &>/dev/null; then
  die "apt-get required"
fi

export DEBIAN_FRONTEND=noninteractive
log "apt install frr…"
apt-get update -qq || true
apt-get install -y -qq frr || die "apt install frr failed"

DAEMONS="/etc/frr/daemons"
if [[ -f "${DAEMONS}" ]]; then
  log "enable zebra + staticd in ${DAEMONS}"
  sed -i 's/^zebra=no/zebra=yes/' "${DAEMONS}"
  sed -i 's/^staticd=no/staticd=yes/' "${DAEMONS}"
  grep -q '^zebra=yes' "${DAEMONS}" || echo 'zebra=yes' >> "${DAEMONS}"
  grep -q '^staticd=yes' "${DAEMONS}" || echo 'staticd=yes' >> "${DAEMONS}"
fi

mkdir -p /etc/frr/frr.conf.d /etc/qosnat2/frr
if [[ ! -f /etc/frr/frr.conf.d/qosnat2.conf ]]; then
  cat > /etc/frr/frr.conf.d/qosnat2.conf <<'EOF'
! qosnat2 managed FRR includes
include /etc/qosnat2/frr/managed-routes.conf
EOF
fi

systemctl enable frr
systemctl restart frr

command -v vtysh &>/dev/null || die "vtysh missing after install"
log "frr installed:"
systemctl is-active frr || true
vtysh -c 'show version' 2>/dev/null | head -3 || true
