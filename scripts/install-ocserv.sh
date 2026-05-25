#!/usr/bin/env bash
# 从源码编译安装 ocserv（OpenConnect VPN 服务端，兼容 AnyConnect 客户端）
# 用法: sudo /opt/qosnat2/scripts/install-ocserv.sh
# 可选: OCSERV_TAG=v1.3.0 OCSERV_PREFIX=/usr/local OCSERV_SYSCONFDIR=/etc/ocserv
set -euo pipefail

OCSERV_REPO="${OCSERV_REPO:-https://gitlab.com/openconnect/ocserv.git}"
OCSERV_TAG="${OCSERV_TAG:-v1.3.0}"
OCSERV_PREFIX="${OCSERV_PREFIX:-/usr/local}"
OCSERV_SYSCONFDIR="${OCSERV_SYSCONFDIR:-/etc/ocserv}"
BUILD_DIR="${BUILD_DIR:-/usr/local/src/ocserv-build}"
OCSERV_BIN="${OCSERV_PREFIX}/sbin/ocserv"
OCPASSWD_BIN="${OCSERV_PREFIX}/sbin/ocpasswd"

log()  { echo "[ocserv-install] $*"; }
warn() { echo "[ocserv-install] WARN: $*" >&2; }
die()  { echo "[ocserv-install] ERROR: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "需要 root"

install_build_deps() {
  if ! command -v apt-get &>/dev/null; then
    die "当前仅提供 apt 系依赖安装；请手动安装 gnutls/libev/libseccomp 等开发包后编译"
  fi
  log "安装编译依赖..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq \
    build-essential git pkg-config autoconf automake libtool \
    gperf gawk \
    libgnutls28-dev libev-dev liblz4-dev libseccomp-dev \
    libreadline-dev libnl-route-3-dev libwrap0-dev \
    libpam0g-dev libcurl4-gnutls-dev libhttp-parser-dev \
    libprotobuf-c-dev protobuf-c-compiler libtalloc-dev \
    libjansson-dev \
    libradcli-dev \
    || die "apt 安装依赖失败"
}

fetch_source() {
  rm -rf "${BUILD_DIR}"
  mkdir -p "${BUILD_DIR}"
  log "克隆 ${OCSERV_REPO} (${OCSERV_TAG})..."
  git clone --depth 1 --branch "${OCSERV_TAG}" "${OCSERV_REPO}" "${BUILD_DIR}/ocserv" \
    || git clone --depth 1 "${OCSERV_REPO}" "${BUILD_DIR}/ocserv"
  cd "${BUILD_DIR}/ocserv"
  if ! git describe --tags --exact-match 2>/dev/null | grep -q .; then
    git checkout "${OCSERV_TAG}" 2>/dev/null || warn "未检出 tag ${OCSERV_TAG}，使用默认分支"
  fi
}

build_install() {
  cd "${BUILD_DIR}/ocserv"
  log "autoreconf..."
  autoreconf -fvi
  log "configure (with RADIUS/radcli if available)..."
  if ! ./configure \
    --prefix="${OCSERV_PREFIX}" \
    --sysconfdir="${OCSERV_SYSCONFDIR}" \
    --localstatedir=/var \
    --enable-legacy-password-hashing; then
    warn "configure 失败，重试 --disable-seccomp"
    ./configure \
      --prefix="${OCSERV_PREFIX}" \
      --sysconfdir="${OCSERV_SYSCONFDIR}" \
      --localstatedir=/var \
      --disable-seccomp \
      --enable-legacy-password-hashing
  fi
  log "make -j$(nproc)..."
  make -j"$(nproc)"
  log "make install..."
  make install
  [[ -x "${OCSERV_BIN}" ]] || die "未找到 ${OCSERV_BIN}"
  command -v "${OCPASSWD_BIN}" &>/dev/null || warn "ocpasswd 未在 PATH"
}

install_systemd() {
  mkdir -p "${OCSERV_SYSCONFDIR}"
  local unit_src=""
  for p in \
    "${BUILD_DIR}/ocserv/doc/systemd/ocserv.service" \
    "${BUILD_DIR}/ocserv/doc/systemd/ocserv@.service" \
    "/lib/systemd/system/ocserv.service"; do
    if [[ -f "$p" ]]; then
      unit_src="$p"
      break
    fi
  done
  if [[ -z "${unit_src}" ]]; then
    warn "未找到 ocserv.service 模板，跳过 systemd"
    return 0
  fi
  local unit_dst="/etc/systemd/system/ocserv.service"
  sed "s|/usr/sbin/ocserv|${OCSERV_BIN}|g; s|/etc/ocserv|${OCSERV_SYSCONFDIR}|g" "${unit_src}" > "${unit_dst}"
  # 去掉 socket activation 行（若存在且二进制路径已改）
  sed -i 's|^ExecStartPre=.*||' "${unit_dst}" 2>/dev/null || true
  systemctl daemon-reload
  log "已安装 ${unit_dst}"
}

seed_config() {
  if [[ ! -f "${OCSERV_SYSCONFDIR}/ocserv.conf" ]]; then
    log "写入默认 ${OCSERV_SYSCONFDIR}/ocserv.conf（可由 qosnatd UI 覆盖）"
    cat > "${OCSERV_SYSCONFDIR}/ocserv.conf" <<'EOF'
# qosnat2 初始模板 — 请通过 Web「VPN → OpenConnect」保存完整配置
auth = "plain[passwd=/etc/ocserv/ocpasswd]"
tcp-port = 443
udp-port = 443
run-as-user = nobody
run-as-group = daemon
socket-file = /var/run/ocserv.sock
server-cert = /etc/ocserv/server-cert.pem
server-key = /etc/ocserv/server-key.pem
ipv4-network = 10.250.0.0
ipv4-netmask = 255.255.255.0
dns = 8.8.8.8
route = default
device = vpns
try-mtu-discovery = true
isolate-workers = true
max-clients = 128
max-same-clients = 2
keepalive = 32400
EOF
    chmod 0644 "${OCSERV_SYSCONFDIR}/ocserv.conf"
  fi
  touch "${OCSERV_SYSCONFDIR}/ocpasswd"
  chmod 0600 "${OCSERV_SYSCONFDIR}/ocpasswd"
  if [[ ! -f "${OCSERV_SYSCONFDIR}/server-cert.pem" ]] && [[ -f /etc/qosnat2/tls.crt ]]; then
    log "复用 /etc/qosnat2/tls.* 为 VPN 证书"
    cp -f /etc/qosnat2/tls.crt "${OCSERV_SYSCONFDIR}/server-cert.pem"
    cp -f /etc/qosnat2/tls.key "${OCSERV_SYSCONFDIR}/server-key.pem"
    chmod 0644 "${OCSERV_SYSCONFDIR}/server-cert.pem"
    chmod 0600 "${OCSERV_SYSCONFDIR}/server-key.pem"
  fi
}

enable_ip_forward() {
  sysctl -w net.ipv4.ip_forward=1 >/dev/null || true
  grep -q '^net.ipv4.ip_forward' /etc/sysctl.d/99-qosnat2.conf 2>/dev/null \
    || echo 'net.ipv4.ip_forward = 1' >> /etc/sysctl.d/99-qosnat2.conf 2>/dev/null || true
}

main() {
  install_build_deps
  fetch_source
  build_install
  install_systemd
  seed_config
  enable_ip_forward
  log "完成。二进制: ${OCSERV_BIN}"
  if ldd "${OCSERV_BIN}" 2>/dev/null | grep -qE 'radcli|radiusclient'; then
    log "RADIUS: 已链接 radcli/radiusclient"
  else
    warn "RADIUS: 未检测到 radcli 链接；请确认已安装 libradcli-dev 后重新运行本脚本"
  fi
  log "下一步: 在 qosnat2 Web「VPN → OpenConnect」配置认证（本地或 RADIUS）与证书，并 Apply。"
  "${OCSERV_BIN}" --version 2>/dev/null || true
}

main "$@"
