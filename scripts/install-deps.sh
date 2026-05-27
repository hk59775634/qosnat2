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
