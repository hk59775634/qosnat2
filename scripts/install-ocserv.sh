#!/usr/bin/env bash
# 从源码编译安装 ocserv（OpenConnect VPN 服务端，兼容 AnyConnect 客户端）
# 用法: sudo /opt/qosnat2/scripts/install-ocserv.sh
# 可选: OCSERV_TAG=1.4.2 OCSERV_PREFIX=/usr/local OCSERV_SYSCONFDIR=/etc/ocserv
set -euo pipefail

OCSERV_REPO="${OCSERV_REPO:-https://gitlab.com/openconnect/ocserv.git}"
OCSERV_TAG="${OCSERV_TAG:-1.4.2}"
OCSERV_PREFIX="${OCSERV_PREFIX:-/usr/local}"
OCSERV_SYSCONFDIR="${OCSERV_SYSCONFDIR:-/etc/ocserv}"
BUILD_DIR="${BUILD_DIR:-/usr/local/src/ocserv-build}"
OCSERV_BIN="${OCSERV_PREFIX}/sbin/ocserv"
MESON_BUILD_DIR="${MESON_BUILD_DIR:-build}"

log()  { echo "[ocserv-install] $*"; }
warn() { echo "[ocserv-install] WARN: $*" >&2; }
die()  { echo "[ocserv-install] ERROR: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "需要 root"

# 避免从已删除的工作目录执行 git/apt（UI 后台触发时 cwd 可能无效）
cd / || true

install_build_deps() {
  if ! command -v apt-get &>/dev/null; then
    die "当前仅提供 apt 系依赖安装；请手动安装编译依赖后重试"
  fi
  log "安装编译依赖..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq \
    build-essential git pkg-config \
    meson ninja-build ipcalc libtasn1-bin \
    gperf gawk \
    autoconf automake libtool \
    libgnutls28-dev libev-dev liblz4-dev libseccomp-dev \
    libreadline-dev libnl-route-3-dev libwrap0-dev \
    libpam0g-dev libcurl4-gnutls-dev \
    libprotobuf-c-dev protobuf-c-compiler libtalloc-dev \
    libjansson-dev libradcli-dev \
    || die "apt 安装依赖失败"
}

fetch_source() {
  cd /
  rm -rf "${BUILD_DIR}"
  mkdir -p "${BUILD_DIR}"
  local tag="${OCSERV_TAG}"
  local alt=""
  if [[ "${tag}" == v* ]]; then
    alt="${tag#v}"
  else
    alt="v${tag}"
  fi
  log "克隆 ${OCSERV_REPO} (tag=${tag})..."
  if git clone --depth 1 --branch "${tag}" "${OCSERV_REPO}" "${BUILD_DIR}/ocserv" 2>/dev/null; then
    :
  elif [[ -n "${alt}" ]] && git clone --depth 1 --branch "${alt}" "${OCSERV_REPO}" "${BUILD_DIR}/ocserv" 2>/dev/null; then
    tag="${alt}"
    log "使用 tag ${tag}"
  else
    warn "浅克隆 tag 失败，完整克隆后检出..."
    git clone "${OCSERV_REPO}" "${BUILD_DIR}/ocserv" || die "git clone failed"
    cd "${BUILD_DIR}/ocserv"
    git checkout "${tag}" 2>/dev/null \
      || { [[ -n "${alt}" ]] && git checkout "${alt}"; } 2>/dev/null \
      || die "无法检出 tag ${OCSERV_TAG}"
    cd - >/dev/null
  fi
  cd "${BUILD_DIR}/ocserv"
  if [[ -f .gitmodules ]] && [[ -d .git ]]; then
    log "初始化 git submodules..."
    git submodule update --init --recursive 2>/dev/null || warn "submodule 初始化失败（可能不影响编译）"
  fi
  if [[ -f meson.build ]]; then
    log "源码版本: $(meson introspect --project-version meson.build 2>/dev/null || echo "${tag}") (Meson)"
  elif [[ -f configure.ac ]]; then
    log "源码版本: ${tag} (Autotools)"
  else
    die "源码树无效：缺少 meson.build 与 configure.ac"
  fi
}

build_install_autotools() {
  log "使用 Autotools 编译..."
  autoreconf -fvi
  local cfg_args=(
    --prefix="${OCSERV_PREFIX}"
    --sysconfdir="${OCSERV_SYSCONFDIR}"
    --localstatedir=/var
    --enable-legacy-password-hashing
  )
  if ! ./configure "${cfg_args[@]}"; then
    warn "configure 失败，重试 --disable-seccomp"
    ./configure "${cfg_args[@]}" --disable-seccomp
  fi
  make -j"$(nproc)"
  make install
}

build_install_meson() {
  log "使用 Meson 编译..."
  rm -rf "${MESON_BUILD_DIR}"
  local meson_args=(
    setup "${MESON_BUILD_DIR}"
    --prefix="${OCSERV_PREFIX}"
    --sysconfdir="${OCSERV_SYSCONFDIR}"
    -Dradius=enabled
  )
  if ! meson "${meson_args[@]}"; then
    warn "Meson 启用 radius 失败，重试 -Dradius=auto"
    meson setup "${MESON_BUILD_DIR}" \
      --prefix="${OCSERV_PREFIX}" \
      --sysconfdir="${OCSERV_SYSCONFDIR}" \
      -Dradius=auto
  fi
  ninja -C "${MESON_BUILD_DIR}"
  ninja -C "${MESON_BUILD_DIR}" install
}

build_install() {
  cd "${BUILD_DIR}/ocserv"
  if [[ -f meson.build ]]; then
    build_install_meson
  elif [[ -f configure.ac ]]; then
    build_install_autotools
  else
    die "无法识别构建系统"
  fi
  [[ -x "${OCSERV_BIN}" ]] || die "未找到 ${OCSERV_BIN}"
  if command -v "${OCSERV_PREFIX}/bin/ocpasswd" &>/dev/null; then
    log "ocpasswd: ${OCSERV_PREFIX}/bin/ocpasswd"
  elif command -v "${OCSERV_PREFIX}/sbin/ocpasswd" &>/dev/null; then
    log "ocpasswd: ${OCSERV_PREFIX}/sbin/ocpasswd"
  else
    warn "ocpasswd 未在 ${OCSERV_PREFIX}/bin 或 sbin"
  fi
}

install_systemd() {
  mkdir -p "${OCSERV_SYSCONFDIR}"
  local unit_src=""
  for p in \
    "${BUILD_DIR}/ocserv/doc/systemd/standalone/ocserv.service" \
    "${BUILD_DIR}/ocserv/doc/systemd/socket-activated/ocserv.service" \
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
  sed -i 's|^ExecStartPre=.*||' "${unit_dst}" 2>/dev/null || true
  systemctl daemon-reload
  log "已安装 ${unit_dst}（来自 $(basename "$(dirname "${unit_src}")")/$(basename "${unit_src}")）"
}

seed_config() {
  mkdir -p "${OCSERV_SYSCONFDIR}/certs"
  if [[ ! -f "${OCSERV_SYSCONFDIR}/ocserv.conf" ]]; then
    log "写入默认 ${OCSERV_SYSCONFDIR}/ocserv.conf（可由 qosnatd UI 覆盖）"
    cat > "${OCSERV_SYSCONFDIR}/ocserv.conf" <<EOF
# qosnat2 初始模板 — 请通过 Web「VPN → OpenConnect」保存完整配置
auth = "plain[passwd=${OCSERV_SYSCONFDIR}/ocpasswd]"
tcp-port = 443
udp-port = 443
run-as-user = nobody
run-as-group = daemon
socket-file = /var/run/ocserv-socket
server-cert = ${OCSERV_SYSCONFDIR}/certs/server-cert.pem
server-key = ${OCSERV_SYSCONFDIR}/certs/server-key.pem
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
  if [[ ! -f "${OCSERV_SYSCONFDIR}/certs/server-cert.pem" ]] && [[ -f /etc/qosnat2/tls.crt ]]; then
    log "复用 /etc/qosnat2/tls.* 为 VPN 证书"
    cp -f /etc/qosnat2/tls.crt "${OCSERV_SYSCONFDIR}/certs/server-cert.pem"
    cp -f /etc/qosnat2/tls.key "${OCSERV_SYSCONFDIR}/certs/server-key.pem"
    chmod 0644 "${OCSERV_SYSCONFDIR}/certs/server-cert.pem"
    chmod 0600 "${OCSERV_SYSCONFDIR}/certs/server-key.pem"
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
    log "RADIUS: 已链接 radcli"
  else
    warn "RADIUS: 未检测到 radcli；检查 libradcli-dev 与 meson -Dradius"
  fi
  log "下一步: Web「VPN → OpenConnect」配置并 Apply"
  "${OCSERV_BIN}" --version 2>/dev/null || true
}

main "$@"
