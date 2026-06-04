# shellcheck shell=bash
# qosnat2 一键安装 / deploy 共用的 apt 依赖列表（Ubuntu 24.04）

# 控制面编译、数据面、Web 构建、诊断与 VPN 相关工具
QOSNAT_APT_PACKAGES=(
  ca-certificates
  curl
  git
  gnupg
  iproute2
  nftables
  golang-go
  clang
  llvm
  libbpf-dev
  make
  pkg-config
  build-essential
  wireguard-tools
  tcpdump
  conntrack
  dnsmasq
  libdbus-1-dev
  libidn2-dev
  nettle-dev
  libnftables-dev
  unbound
  nodejs
  npm
)

# NAT64 数据面（内核模块需与运行内核匹配，失败时请安装 jool-dkms）
QOSNAT_APT_PACKAGES_OPTIONAL=(
  jool-tools
)

qosnat_apt_install_packages() {
  if ! command -v apt-get &>/dev/null; then
    echo "ERROR: 未找到 apt-get，无法自动安装依赖" >&2
    return 1
  fi
  export DEBIAN_FRONTEND=noninteractive
  apt-get update -qq
  apt-get install -y -qq "${QOSNAT_APT_PACKAGES[@]}"
  apt-get install -y -qq "${QOSNAT_APT_PACKAGES_OPTIONAL[@]}" || true
}

# 安装带 chnroutes 补丁的 dnsmasq（优先 release 预编译包，否则源码编译）
qosnat_install_dnsmasq_chnroutes() {
  local lib="${QOSNAT_LIB:-/usr/local/lib/qosnat2}"
  local install_bin="${ROOT_DIR:-$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)}/scripts/install-dnsmasq-chnroutes-binary.sh"
  if command -v dnsmasq &>/dev/null && dnsmasq --help 2>&1 | grep -q chnroutes-file; then
    return 0
  fi
  if [[ -f "${lib}/dnsmasq-chnroutes" ]] && "${lib}/dnsmasq-chnroutes" --help 2>&1 | grep -q chnroutes-file; then
    bash "${install_bin}" "${lib}/dnsmasq-chnroutes"
    return 0
  fi
  local prebuilt="${QOSNAT_ROOT:-}/dist/lib/dnsmasq-chnroutes"
  if [[ -z "${QOSNAT_ROOT:-}" ]]; then
    prebuilt="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/dist/lib/dnsmasq-chnroutes"
  fi
  if [[ -f "${prebuilt}" ]]; then
    bash "${install_bin}" "${prebuilt}"
    return 0
  fi
  local script="${QOSNAT_ROOT:-}/scripts/build-dnsmasq-chnroutes.sh"
  if [[ -z "${QOSNAT_ROOT:-}" ]]; then
    script="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/scripts/build-dnsmasq-chnroutes.sh"
  fi
  [[ -f "${script}" ]] || script="$(dirname "${BASH_SOURCE[0]}")/build-dnsmasq-chnroutes.sh"
  if [[ ! -f "${script}" ]]; then
    echo "WARN: missing ${script}, skip patched dnsmasq build" >&2
    return 0
  fi
  chmod +x "${script}" "${install_bin}"
  bash "${script}"
}
