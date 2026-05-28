#!/usr/bin/env bash
# 安装 ocserv（OpenConnect VPN 服务端）
# 生产环境默认使用预编译 release 包；开发环境可用 --method source 从源码编译。
#
# 用法:
#   sudo /opt/qosnat2/scripts/install-ocserv.sh
#   sudo /opt/qosnat2/scripts/install-ocserv.sh --method release --version 2026052801
#   sudo /opt/qosnat2/scripts/install-ocserv.sh --method source   # 仅开发/构建机
#
# 环境变量:
#   INSTALL_METHOD=release|source
#   OCSERV_VERSION=2026052801
#   OCSERV_VERSIONS_URL=...   # 默认 GitHub releases/ocserv-versions.json
#   OCSERV_DOWNLOAD_URL=...   # 可选，覆盖默认 GitHub release URL
#   OCSERV_PREFIX=/usr/local OCSERV_SYSCONFDIR=/etc/ocserv  # source 编译用
set -euo pipefail

OCSERV_REPO="${OCSERV_REPO:-https://gitlab.com/openconnect/ocserv.git}"
OCSERV_TAG="${OCSERV_TAG:-}"
OCSERV_VERSION="${OCSERV_VERSION:-${OCSERV_TAG}}"
OCSERV_VERSIONS_URL="${OCSERV_VERSIONS_URL:-https://raw.githubusercontent.com/hk59775634/qosnat2/main/releases/ocserv-versions.json}"
OCSERV_PREFIX="${OCSERV_PREFIX:-/usr/local}"
OCSERV_SYSCONFDIR="${OCSERV_SYSCONFDIR:-/etc/ocserv}"
BUILD_DIR="${BUILD_DIR:-/usr/local/src/ocserv-build}"
OCSERV_BIN="${OCSERV_PREFIX}/sbin/ocserv"
MESON_BUILD_DIR="${MESON_BUILD_DIR:-build}"
INSTALL_METHOD="${INSTALL_METHOD:-release}"
GITHUB_REPO="${GITHUB_REPO:-hk59775634/qosnat2}"
RELEASE_ASSET="${RELEASE_ASSET:-ocserv-linux-amd64.tar.gz}"

log()  { echo "[ocserv-install] $*"; }
warn() { echo "[ocserv-install] WARN: $*" >&2; }
die()  { echo "[ocserv-install] ERROR: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "需要 root"
cd / || true

normalize_version() {
  local v="${1:-}"
  v="${v#ocserv-}"
  v="${v#v}"
  echo "${v}"
}

detect_release_version() {
  if [[ -n "${OCSERV_VERSION}" ]]; then
    normalize_version "${OCSERV_VERSION}"
    return 0
  fi
  local id
  if command -v jq >/dev/null 2>&1; then
    id="$(curl -fsSL "${OCSERV_VERSIONS_URL}" | jq -r '.versions[0].id // .versions[0].tag // empty' | head -n1)"
    id="$(normalize_version "${id}")"
  else
    id="$(curl -fsSL "${OCSERV_VERSIONS_URL}" | python3 -c "import json,sys; d=json.load(sys.stdin); v=d.get('versions') or []; e=v[0] if v else {}; print((e.get('id') or e.get('tag') or '').replace('ocserv-','').replace('v',''))" 2>/dev/null || true)"
  fi
  [[ -n "${id}" ]] || die "无法从版本清单获取版本，请设置 OCSERV_VERSION 或检查 ${OCSERV_VERSIONS_URL}"
  echo "${id}"
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --method) INSTALL_METHOD="${2:-release}"; shift 2 ;;
      --version) OCSERV_VERSION="${2:-}"; shift 2 ;;
      --url) OCSERV_DOWNLOAD_URL="${2:-}"; shift 2 ;;
      -h|--help)
        sed -n '1,20p' "$0"
        exit 0
        ;;
      *) die "未知参数: $1" ;;
    esac
  done
  OCSERV_VERSION="$(normalize_version "${OCSERV_VERSION}")"
  if [[ "${INSTALL_METHOD}" == "release" && -z "${OCSERV_VERSION}" ]]; then
    OCSERV_VERSION="$(detect_release_version)"
  fi
  if [[ "${INSTALL_METHOD}" == "release" ]]; then
    [[ -n "${OCSERV_VERSION}" ]] || die "release 安装需要版本号（--version 或版本清单）"
    [[ "${OCSERV_VERSION}" =~ ^[0-9]{10}$ ]] || die "版本号格式应为 YYYYMMDDNN（10 位数字）: ${OCSERV_VERSION}"
  elif [[ -z "${OCSERV_VERSION}" ]]; then
    OCSERV_VERSION="1.4.2"
  fi
}

default_release_url() {
  local ver tag
  ver="$(normalize_version "${OCSERV_VERSION}")"
  tag="ocserv-${ver}"
  echo "https://github.com/${GITHUB_REPO}/releases/download/${tag}/${RELEASE_ASSET}"
}

install_release() {
  local url tmp
  url="${OCSERV_DOWNLOAD_URL:-$(default_release_url)}"
  tmp="$(mktemp -d /tmp/ocserv-release.XXXXXX)"
  trap 'rm -rf "${tmp}"' RETURN
  log "下载 release: ${url}"
  command -v curl >/dev/null || die "需要 curl"
  curl -fL --retry 3 --retry-delay 2 "${url}" -o "${tmp}/pkg.tar.gz"
  tar -xzf "${tmp}/pkg.tar.gz" -C "${tmp}"
  [[ -f "${tmp}/bin/ocserv" ]] || die "release 包缺少 bin/ocserv"
  install -d /usr/local/sbin /usr/local/bin
  install -m 0755 "${tmp}/bin/ocserv" /usr/local/sbin/ocserv
  for tool in occtl ocpasswd; do
    if [[ -f "${tmp}/bin/${tool}" ]]; then
      install -m 0755 "${tmp}/bin/${tool}" "/usr/local/bin/${tool}"
    fi
  done
  if [[ -f "${tmp}/systemd/ocserv.service" ]]; then
    sed "s|/usr/sbin/ocserv|/usr/local/sbin/ocserv|g; s|/etc/ocserv|${OCSERV_SYSCONFDIR}|g" \
      "${tmp}/systemd/ocserv.service" > /etc/systemd/system/ocserv.service
    sed -i 's|^ExecStartPre=.*||' /etc/systemd/system/ocserv.service 2>/dev/null || true
    systemctl daemon-reload
  fi
  seed_config
  enable_ip_forward
  install -d /var/lib/qosnat2
  echo "${OCSERV_VERSION}" > /var/lib/qosnat2/ocserv-release-tag
  log "release 安装完成: /usr/local/sbin/ocserv (${OCSERV_VERSION})"
  /usr/local/sbin/ocserv --version 2>/dev/null || true
}

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
    git submodule update --init --recursive 2>/dev/null || warn "submodule 初始化失败"
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
  if ! meson setup "${MESON_BUILD_DIR}" \
    --prefix="${OCSERV_PREFIX}" \
    --sysconfdir="${OCSERV_SYSCONFDIR}" \
    -Dradius=enabled; then
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
}

install_systemd_from_source() {
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
  sed "s|/usr/sbin/ocserv|${OCSERV_BIN}|g; s|/etc/ocserv|${OCSERV_SYSCONFDIR}|g" "${unit_src}" > /etc/systemd/system/ocserv.service
  sed -i 's|^ExecStartPre=.*||' /etc/systemd/system/ocserv.service 2>/dev/null || true
  systemctl daemon-reload
}

seed_config() {
  mkdir -p "${OCSERV_SYSCONFDIR}/certs"
  if [[ ! -f "${OCSERV_SYSCONFDIR}/ocserv.conf" ]]; then
    log "写入默认 ${OCSERV_SYSCONFDIR}/ocserv.conf"
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

install_source() {
  OCSERV_TAG="${OCSERV_VERSION}"
  install_build_deps
  fetch_source
  build_install
  install_systemd_from_source
  seed_config
  enable_ip_forward
  install -d /var/lib/qosnat2
  echo "${OCSERV_VERSION}" > /var/lib/qosnat2/ocserv-release-tag
  log "源码安装完成: ${OCSERV_BIN}"
  if ldd "${OCSERV_BIN}" 2>/dev/null | grep -qE 'radcli|radiusclient'; then
    log "RADIUS: 已链接 radcli"
  else
    warn "RADIUS: 未检测到 radcli"
  fi
  "${OCSERV_BIN}" --version 2>/dev/null || true
}

main() {
  parse_args "$@"
  case "${INSTALL_METHOD}" in
    release|binary)
      install_release
      ;;
    source)
      install_source
      ;;
    *)
      die "未知 INSTALL_METHOD=${INSTALL_METHOD}（可用 release 或 source）"
      ;;
  esac
}

main "$@"
