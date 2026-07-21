#!/usr/bin/env bash
# 安装 ocserv（OpenConnect VPN 服务端）— 官方 1.5.0 源码 + Route B SPEC-01 + DST SPEC-02 补丁。
#
# 用法:
#   sudo /opt/qosnat2/scripts/install-ocserv.sh
#   sudo /opt/qosnat2/scripts/install-ocserv.sh --version 1.5.0
#
# 环境变量:
#   OCSERV_TAG / OCSERV_VERSION   官方 tag（默认且仅验证 1.5.0）
#   OCSERV_MIRROR_REPO            上游镜像（默认 github.com/openconnect/ocserv；补丁在 patches/ocserv）
#   OCSERV_GITLAB_REPO            官方 GitLab 回退
#   OCSERV_PREFIX=/usr/local OCSERV_SYSCONFDIR=/etc/ocserv
#   PATCH_DIR=.../patches/ocserv   SPEC-01/DST 脚本目录（默认仓库内 patches/ocserv）
#   OCSERV_ALLOW_UNPATCHED=1       允许非 1.5.0（跳过补丁，仅开发用）
set -euo pipefail

OCSERV_MIRROR_REPO="${OCSERV_MIRROR_REPO:-https://github.com/openconnect/ocserv.git}"
OCSERV_MIRROR_SLUG="${OCSERV_MIRROR_SLUG:-openconnect/ocserv}"
OCSERV_GITHUB_REPO="${OCSERV_GITHUB_REPO:-https://github.com/openconnect/ocserv.git}"
OCSERV_GITLAB_REPO="${OCSERV_GITLAB_REPO:-https://gitlab.com/openconnect/ocserv.git}"
GH_PROXY_V4="${GH_PROXY_V4:-https://v4.gh-proxy.org/}"
GH_PROXY_CDN="${GH_PROXY_CDN:-https://cdn.gh-proxy.org/}"
OCSERV_TAG="${OCSERV_TAG:-1.5.0}"
OCSERV_VERSION="${OCSERV_VERSION:-${OCSERV_TAG}}"
OCSERV_PREFIX="${OCSERV_PREFIX:-/usr/local}"
OCSERV_SYSCONFDIR="${OCSERV_SYSCONFDIR:-/etc/ocserv}"
BUILD_DIR="${BUILD_DIR:-/usr/local/src/ocserv-build}"
PATCH_DIR="${PATCH_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/patches/ocserv}"
SPEC01_SCRIPT="${SPEC01_SCRIPT:-${PATCH_DIR}/apply-spec01-edits.py}"
DST_SCRIPT="${DST_SCRIPT:-${PATCH_DIR}/apply-dst-edits.py}"
OCSERV_BIN="${OCSERV_PREFIX}/sbin/ocserv"
OCSERV_WORKER_BIN="${OCSERV_PREFIX}/sbin/ocserv-worker"
MESON_BUILD_DIR="${MESON_BUILD_DIR:-build}"
OCSERV_SPEC01_BASELINE="1.5.0"

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

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --method)
        warn "已忽略 --method（仅支持源码安装）"
        shift 2
        ;;
      --version) OCSERV_VERSION="${2:-}"; shift 2 ;;
      --url)
        warn "已忽略 --url（不再使用 GitHub 预编译包）"
        shift 2
        ;;
      -h|--help)
        sed -n '1,15p' "$0"
        exit 0
        ;;
      *) die "未知参数: $1" ;;
    esac
  done
  OCSERV_VERSION="$(normalize_version "${OCSERV_VERSION}")"
  [[ -n "${OCSERV_VERSION}" ]] || OCSERV_VERSION="1.5.0"
}

install_build_deps() {
  if ! command -v apt-get &>/dev/null; then
    die "当前仅提供 apt 系依赖安装；请手动安装编译依赖后重试"
  fi
  log "安装编译依赖..."
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y --no-install-recommends \
    build-essential meson ninja-build pkg-config git curl xz-utils python3 \
    libgnutls28-dev libev-dev libreadline-dev libseccomp-dev \
    libnl-route-3-dev libnl-genl-3-dev libtalloc-dev libhttp-parser-dev \
    libprotobuf-c-dev libpam0g-dev libradcli-dev \
    libcurl4-gnutls-dev liboath-dev libmaxminddb-dev \
    liblz4-dev libbrotli-dev libnghttp2-dev \
    gperf ipcalc libjansson-dev libwrap0-dev protobuf-c-compiler libtasn1-bin \
    autoconf automake libtool
}

finalize_extracted_source() {
  local extracted
  extracted="$(find "${BUILD_DIR}" -maxdepth 1 -mindepth 1 -type d | head -1)"
  [[ -n "${extracted}" ]] || die "解压后未找到源码目录"
  mv "${extracted}" "${BUILD_DIR}/ocserv"
}

try_download_archive() {
  local url="$1"
  local out="$2"
  log "下载源码包 ${url} ..."
  curl -fsSL --connect-timeout 25 --max-time 600 -o "${out}" "${url}"
}

fetch_source() {
  local tag="${OCSERV_VERSION}"
  rm -rf "${BUILD_DIR}"
  mkdir -p "${BUILD_DIR}"
  local archive_out="${BUILD_DIR}/ocserv-src.tar"
  local mirror_codeload="https://codeload.github.com/${OCSERV_MIRROR_SLUG}/tar.gz/refs/tags/${tag}"
  local mirror_archive="https://github.com/${OCSERV_MIRROR_SLUG}/archive/refs/tags/${tag}.tar.gz"
  local gitlab_archive="${OCSERV_GITLAB_REPO%/}/-/archive/${tag}/ocserv-${tag}.tar.gz"
  local infradead_archive="https://www.infradead.org/ocserv/download/ocserv-${tag}.tar.xz"
  local archive_urls=(
    "${GH_PROXY_V4}${mirror_codeload}|${archive_out}.gz|gzip"
    "${GH_PROXY_CDN}${mirror_codeload}|${archive_out}.gz|gzip"
    "${GH_PROXY_V4}${mirror_archive}|${archive_out}.gz|gzip"
    "${GH_PROXY_CDN}${mirror_archive}|${archive_out}.gz|gzip"
    "${gitlab_archive}|${archive_out}.gz|gzip"
    "${infradead_archive}|${archive_out}.xz|xz"
  )
  local entry url out fmt
  for entry in "${archive_urls[@]}"; do
    IFS='|' read -r url out fmt <<< "${entry}"
    if try_download_archive "${url}" "${out}" 2>/dev/null; then
      case "${fmt}" in
        gzip) tar -xzf "${out}" -C "${BUILD_DIR}" ;;
        xz) tar -xJf "${out}" -C "${BUILD_DIR}" ;;
        *) die "unknown archive format: ${fmt}" ;;
      esac
      rm -f "${out}"
      finalize_extracted_source
      log "源码包下载完成"
      return 0
    fi
    warn "下载失败: ${url}"
    rm -f "${out}"
  done

  local clone_urls=(
    "${GH_PROXY_V4}${OCSERV_MIRROR_REPO}"
    "${GH_PROXY_CDN}${OCSERV_MIRROR_REPO}"
    "${OCSERV_MIRROR_REPO}"
    "${OCSERV_GITLAB_REPO}"
  )
  for url in "${clone_urls[@]}"; do
    log "克隆 ${url} (tag=${tag})..."
    if git clone --depth 1 --branch "${tag}" "${url}" "${BUILD_DIR}/ocserv" 2>/dev/null; then
      return 0
    fi
  done
  for url in "${clone_urls[@]}"; do
    warn "浅克隆失败，完整克隆 ${url}..."
    if git clone "${url}" "${BUILD_DIR}/ocserv" 2>/dev/null; then
      (
        cd "${BUILD_DIR}/ocserv"
        git checkout "${tag}" 2>/dev/null || die "无法检出 tag ${tag}"
      )
      return 0
    fi
  done
  die "无法获取 ocserv 源码（tag=${tag}）"
}

apply_spec01() {
  if [[ "${OCSERV_VERSION}" != "${OCSERV_SPEC01_BASELINE}" ]]; then
    if [[ "${OCSERV_ALLOW_UNPATCHED:-}" == "1" ]]; then
      warn "跳过 SPEC-01（OCSERV_ALLOW_UNPATCHED=1，version=${OCSERV_VERSION}）"
      return 0
    fi
    die "Route B 生产安装仅支持 ocserv ${OCSERV_SPEC01_BASELINE}（当前: ${OCSERV_VERSION}）；开发跳过补丁请设 OCSERV_ALLOW_UNPATCHED=1"
  fi
  [[ -f "${SPEC01_SCRIPT}" ]] || die "缺少 SPEC-01 脚本: ${SPEC01_SCRIPT}"
  command -v python3 >/dev/null || die "需要 python3 以应用 SPEC-01"
  log "应用完整 SPEC-01（ocserv-tunnel apply-spec01-edits.py）…"
  python3 "${SPEC01_SCRIPT}" "${BUILD_DIR}/ocserv"
}

apply_dst() {
  if [[ "${OCSERV_VERSION}" != "${OCSERV_SPEC01_BASELINE}" ]]; then
    if [[ "${OCSERV_ALLOW_UNPATCHED:-}" == "1" ]]; then
      warn "跳过 SPEC-02 DST（OCSERV_ALLOW_UNPATCHED=1，version=${OCSERV_VERSION}）"
      return 0
    fi
    die "DST 生产安装仅支持 ocserv ${OCSERV_SPEC01_BASELINE}（当前: ${OCSERV_VERSION}）"
  fi
  [[ -f "${DST_SCRIPT}" ]] || die "缺少 SPEC-02 DST 脚本: ${DST_SCRIPT}"
  command -v python3 >/dev/null || die "需要 python3 以应用 DST"
  log "应用 SPEC-02 DST（dynamic-split-*-domains）…"
  python3 "${DST_SCRIPT}" "${BUILD_DIR}/ocserv"
}

# Explicitly install main + worker so isolate-workers never keeps a stale unpatched worker.
install_binaries() {
  local built_ocserv="${BUILD_DIR}/ocserv/${MESON_BUILD_DIR}/src/ocserv"
  local built_worker="${BUILD_DIR}/ocserv/${MESON_BUILD_DIR}/src/ocserv-worker"
  if [[ ! -x "${built_ocserv}" ]]; then
    built_ocserv="$(find "${BUILD_DIR}/ocserv/${MESON_BUILD_DIR}" -type f -executable -name ocserv | head -1)"
  fi
  if [[ ! -x "${built_worker}" ]]; then
    built_worker="$(find "${BUILD_DIR}/ocserv/${MESON_BUILD_DIR}" -type f -executable -name ocserv-worker | head -1)"
  fi
  [[ -n "${built_ocserv}" && -x "${built_ocserv}" ]] || die "构建产物缺少 ocserv"
  [[ -n "${built_worker}" && -x "${built_worker}" ]] || die "构建产物缺少 ocserv-worker（SPEC-01 依赖 worker 侧 group-access）"
  install -d "${OCSERV_PREFIX}/sbin"
  install -m 0755 "${built_ocserv}" "${OCSERV_BIN}"
  install -m 0755 "${built_worker}" "${OCSERV_WORKER_BIN}"
  log "已安装 ${OCSERV_BIN} 与 ${OCSERV_WORKER_BIN}"
}

verify_spec01_binaries() {
  if [[ "${OCSERV_VERSION}" != "${OCSERV_SPEC01_BASELINE}" && "${OCSERV_ALLOW_UNPATCHED:-}" == "1" ]]; then
    return 0
  fi
  local missing=0
  if ! grep -aFq "radius_auth_bind_group" "${OCSERV_BIN}"; then
    warn "ocserv 缺少符号字符串 radius_auth_bind_group"
    missing=1
  fi
  if ! grep -aFq "TunnelGroupName" "${OCSERV_BIN}"; then
    warn "ocserv 缺少 TunnelGroupName 相关字符串"
    missing=1
  fi
  if ! grep -aFq "parse_group_access_url" "${OCSERV_WORKER_BIN}"; then
    warn "ocserv-worker 缺少 parse_group_access_url（OpenConnect <group-access>）"
    missing=1
  fi
  if ! grep -aFq "<group-access>" "${OCSERV_WORKER_BIN}"; then
    warn "ocserv-worker 缺少 <group-access> 解析"
    missing=1
  fi
  [[ "${missing}" -eq 0 ]] || die "SPEC-01 校验失败：二进制未完整打补丁，拒绝安装残缺产物"
  log "SPEC-01 校验通过（ocserv + ocserv-worker）"
}

verify_dst_binaries() {
  if [[ "${OCSERV_VERSION}" != "${OCSERV_SPEC01_BASELINE}" && "${OCSERV_ALLOW_UNPATCHED:-}" == "1" ]]; then
    return 0
  fi
  local missing=0
  # Prefer grep -aF (avoids pipefail+grep -q SIGPIPE false negatives).
  if ! grep -aFq "dynamic-split-include-domains" "${OCSERV_BIN}"; then
    warn "ocserv 缺少 dynamic-split-include-domains"
    missing=1
  fi
  if ! grep -aFq "dynamic-split-exclude-domains" "${OCSERV_BIN}"; then
    warn "ocserv 缺少 dynamic-split-exclude-domains"
    missing=1
  fi
  if ! grep -aFq "X-CSTP-Post-Auth-XML" "${OCSERV_WORKER_BIN}"; then
    warn "ocserv-worker 缺少 X-CSTP-Post-Auth-XML"
    missing=1
  fi
  if ! grep -aFq "dynamic-split-include-domains" "${OCSERV_WORKER_BIN}"; then
    warn "ocserv-worker 缺少 dynamic-split-include-domains"
    missing=1
  fi
  if ! grep -aFq "dst_join_domains" "${OCSERV_WORKER_BIN}"; then
    warn "ocserv-worker 缺少 dst_join_domains"
    missing=1
  fi
  [[ "${missing}" -eq 0 ]] || die "SPEC-02 DST 校验失败：二进制未完整打补丁"
  log "SPEC-02 DST 校验通过（ocserv + ocserv-worker）"
}


build_install() {
  cd "${BUILD_DIR}/ocserv"
  if [[ -f meson.build ]]; then
    log "Meson 构建..."
    meson setup "${MESON_BUILD_DIR}" --prefix="${OCSERV_PREFIX}" --sysconfdir="${OCSERV_SYSCONFDIR}"
    ninja -C "${MESON_BUILD_DIR}"
    ninja -C "${MESON_BUILD_DIR}" install
    install_binaries
  elif [[ -f configure.ac || -f configure ]]; then
    log "Autotools 构建..."
    [[ -x ./configure ]] || autoreconf -fi
    ./configure --prefix="${OCSERV_PREFIX}" --sysconfdir="${OCSERV_SYSCONFDIR}"
    make -j"$(nproc)"
    make install
    [[ -x "${OCSERV_BIN}" ]] || die "未找到 ${OCSERV_BIN}"
    [[ -x "${OCSERV_WORKER_BIN}" ]] || die "未找到 ${OCSERV_WORKER_BIN}"
  else
    die "未识别的构建系统"
  fi
  verify_spec01_binaries
  verify_dst_binaries
}

install_systemd_from_source() {
  local src=""
  local candidates=(
    "${BUILD_DIR}/ocserv/doc/systemd/standalone/ocserv.service"
    "${BUILD_DIR}/ocserv/doc/systemd/ocserv.service"
    "${BUILD_DIR}/ocserv/doc/systemd/socket-activated/ocserv.service"
  )
  for c in "${candidates[@]}"; do
    if [[ -f "${c}" ]]; then
      src="${c}"
      break
    fi
  done
  if [[ -n "${src}" ]]; then
    log "安装 systemd 单元（来自 ${src}）..."
    sed "s|/usr/sbin/ocserv|${OCSERV_BIN}|g; s|/usr/local/sbin/ocserv|${OCSERV_BIN}|g; s|/etc/ocserv|${OCSERV_SYSCONFDIR}|g" \
      "${src}" > /etc/systemd/system/ocserv.service
  else
    warn "未找到上游 systemd 模板，写入内置 ocserv.service"
    write_builtin_ocserv_unit
  fi
  systemctl daemon-reload
}

write_builtin_ocserv_unit() {
  cat > /etc/systemd/system/ocserv.service <<EOF
[Unit]
Description=OpenConnect SSL VPN server
Documentation=man:ocserv(8)
After=network-online.target

[Service]
PrivateTmp=true
PIDFile=/run/ocserv.pid
Type=simple
ExecStart=${OCSERV_BIN} --log-stderr --foreground --pid-file /run/ocserv.pid --config ${OCSERV_SYSCONFDIR}/ocserv.conf
ExecReload=/bin/kill -HUP \$MAINPID

[Install]
WantedBy=multi-user.target
EOF
}

seed_config() {
  install -d "${OCSERV_SYSCONFDIR}/certs" "${OCSERV_SYSCONFDIR}/config-per-group"
  if [[ ! -f "${OCSERV_SYSCONFDIR}/ocserv.conf" ]]; then
    cat > "${OCSERV_SYSCONFDIR}/ocserv.conf" <<EOF
auth = "plain[passwd=${OCSERV_SYSCONFDIR}/ocpasswd]"
tcp-port = 443
udp-port = 443
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
  apply_spec01
  apply_dst
  build_install
  install_systemd_from_source
  seed_config
  enable_ip_forward
  install -d /var/lib/qosnat2
  echo "${OCSERV_VERSION}" > /var/lib/qosnat2/ocserv-release-tag
  log "源码安装完成: ${OCSERV_BIN} + ${OCSERV_WORKER_BIN} (${OCSERV_VERSION}, SPEC-01+DST)"
  if ldd "${OCSERV_BIN}" 2>/dev/null | grep -qE 'radcli|radiusclient'; then
    log "RADIUS: 已链接 radcli"
  else
    warn "RADIUS: 未检测到 radcli"
  fi
  "${OCSERV_BIN}" --version 2>/dev/null || true
}

main() {
  parse_args "$@"
  install_source
}

main "$@"
