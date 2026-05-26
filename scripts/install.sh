#!/usr/bin/env bash
# qosnat2 一键安装（类似 aaPanel：curl | bash）
#
# 说明：一键安装功能仅在 Ubuntu 24.04 上完成安装验证，强烈推荐使用 Ubuntu 24.04。
#
# 默认 HTTP 安装：
#   curl -ksSL https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh | bash
#
# 启用 HTTPS（公网 IP + Let's Encrypt 短期 IP 证书，HTTP-01 需 80 端口可达）：
#   curl -ksSL https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh | bash -s -- ipssl
#
# 可选环境变量：
#   ACME_EMAIL=...               Let's Encrypt 账户邮箱（ipssl；未设置时默认 hk59775634@gmail.com）
#   PUBLIC_IP=1.2.3.4            指定公网 IPv4（默认自动探测）
#   ACME_STAGING=1               使用 LE 测试环境
#   QOSNAT_SKIP_OS_CHECK=1       非 Ubuntu 24.04 时仍继续（不推荐）
#   QOSNAT_INSTALL_DIR=/opt/qosnat2
#   QOSNAT_REPO=https://github.com/hk59775634/qosnat2.git
#   QOSNAT_BRANCH=main
#
# 卸载见 scripts/uninstall.sh 或 deploy-qos-nat.sh uninstall

set -euo pipefail

QOSNAT_REPO="${QOSNAT_REPO:-https://github.com/hk59775634/qosnat2.git}"
QOSNAT_BRANCH="${QOSNAT_BRANCH:-main}"
QOSNAT_INSTALL_DIR="${QOSNAT_INSTALL_DIR:-/opt/qosnat2}"
DEFAULT_ACME_EMAIL="${DEFAULT_ACME_EMAIL:-hk59775634@gmail.com}"
IPSSL=0

for arg in "$@"; do
  case "${arg,,}" in
    ipssl) IPSSL=1 ;;
  esac
done

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

require_root() {
  [[ "$(id -u)" -eq 0 ]] || die "请使用 root 或 sudo 运行一键安装"
}

check_os_for_one_click() {
  local id version_id pretty
  if [[ ! -f /etc/os-release ]]; then
    die "无法识别操作系统。一键安装仅验证 Ubuntu 24.04，请使用 Ubuntu 24.04。"
  fi
  # shellcheck disable=SC1091
  source /etc/os-release
  id="${ID:-}"
  version_id="${VERSION_ID:-}"
  pretty="${PRETTY_NAME:-${NAME:-unknown} ${version_id}}"

  echo ""
  echo "=========================================="
  echo " qosnat2 一键安装"
  echo " 验证平台: Ubuntu 24.04（推荐）"
  echo " 当前系统: ${pretty}"
  echo "=========================================="
  echo ""

  if [[ "${id}" != "ubuntu" ]]; then
    die "一键安装当前仅在 Ubuntu 24.04 上验证。检测到非 Ubuntu 系统，请改用 Ubuntu 24.04 后重试，或参考文档手动部署。"
  fi

  if [[ "${version_id}" != "24.04" ]]; then
    warn "一键安装功能仅在 Ubuntu 24.04 上完成安装验证。"
    warn "当前为 Ubuntu ${version_id}，强烈建议换用 Ubuntu 24.04 再执行一键安装。"
    warn "若了解风险仍要继续，请设置: export QOSNAT_SKIP_OS_CHECK=1"
    if [[ "${QOSNAT_SKIP_OS_CHECK:-0}" != "1" ]]; then
      die "已中止。请使用 Ubuntu 24.04，或设置 QOSNAT_SKIP_OS_CHECK=1 强制继续。"
    fi
    warn "已设置 QOSNAT_SKIP_OS_CHECK=1，在非验证版本上继续安装…"
  else
    log "系统检查通过: Ubuntu 24.04"
  fi

  command -v apt-get &>/dev/null || die "一键安装需要 apt（Ubuntu）。未找到 apt-get。"
}

install_system_packages() {
  log "安装必要软件包（apt）…"
  if ! DEBIAN_FRONTEND=noninteractive apt-get update -qq; then
    die "apt-get update 失败"
  fi
  # 与 scripts/install-deps.sh 保持一致的包列表（克隆前内联，避免依赖未下载的仓库文件）
  if ! DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    ca-certificates curl git gnupg \
    iproute2 nftables \
    golang-go clang llvm libbpf-dev make pkg-config build-essential \
    wireguard-tools tcpdump conntrack dnsmasq \
    nodejs npm; then
    die "必要软件包安装失败，请检查 apt 源与网络后重试"
  fi
  log "必要软件包已安装"
  need_cmd git
  need_cmd curl
  need_cmd go
  need_cmd npm
}

need_cmd() {
  command -v "$1" &>/dev/null || die "安装后仍缺少命令: $1"
}

clone_or_update() {
  if [[ -d "${QOSNAT_INSTALL_DIR}/.git" ]]; then
    log "更新已有仓库 ${QOSNAT_INSTALL_DIR} …"
    git -C "${QOSNAT_INSTALL_DIR}" fetch --depth 1 origin "${QOSNAT_BRANCH}" 2>/dev/null || true
    git -C "${QOSNAT_INSTALL_DIR}" checkout "${QOSNAT_BRANCH}" 2>/dev/null || true
    git -C "${QOSNAT_INSTALL_DIR}" pull --ff-only origin "${QOSNAT_BRANCH}" 2>/dev/null \
      || warn "git pull 失败，将使用当前检出继续"
  elif [[ -e "${QOSNAT_INSTALL_DIR}" ]]; then
    die "${QOSNAT_INSTALL_DIR} 已存在且不是 git 仓库"
  else
    log "克隆 ${QOSNAT_REPO} -> ${QOSNAT_INSTALL_DIR}"
    git clone --depth 1 --branch "${QOSNAT_BRANCH}" "${QOSNAT_REPO}" "${QOSNAT_INSTALL_DIR}"
  fi
}

detect_public_ipv4() {
  if [[ -n "${PUBLIC_IP:-}" ]]; then
    echo "${PUBLIC_IP}"
    return 0
  fi
  local ip url
  for url in https://api.ipify.org https://ifconfig.me/ip https://ipv4.icanhazip.com; do
    ip="$(curl -4 -fsS --max-time 10 "${url}" 2>/dev/null | tr -d '[:space:]')"
    if [[ "${ip}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
      echo "${ip}"
      return 0
    fi
  done
  ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [[ "${ip}" =~ ^[0-9]+\.[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
    echo "${ip}"
    return 0
  fi
  return 1
}

check_port80_for_ipssl() {
  if command -v ss &>/dev/null && ss -tlnH 2>/dev/null | grep -qE ':80$'; then
    warn "TCP 80 已被占用；IP 证书 HTTP-01 校验需要公网可访问本机 80 端口"
    warn "请停止占用 80 的服务（如 nginx/apache）后重试，或使用无 ipssl 安装"
  fi
}

main() {
  require_root
  check_os_for_one_click
  install_system_packages
  clone_or_update

  export IPSSL
  export ONE_CLICK_INSTALL=1
  if [[ "${IPSSL}" == "1" ]]; then
    export PUBLIC_IP="${PUBLIC_IP:-$(detect_public_ipv4 || true)}"
    [[ -n "${PUBLIC_IP:-}" ]] || die "无法探测公网 IPv4，请设置 PUBLIC_IP=..."
    if [[ -z "${ACME_EMAIL:-}" ]]; then
      export ACME_EMAIL="${DEFAULT_ACME_EMAIL}"
      log "ipssl：ACME_EMAIL 未设置，使用默认 ${ACME_EMAIL}"
    fi
    if [[ "${ACME_EMAIL}" == *@example.com ]] || [[ "${ACME_EMAIL}" == *@example.org ]]; then
      die "ACME_EMAIL 不能使用 example.com/example.org（Let's Encrypt 会拒绝），请填写真实邮箱"
    fi
    check_port80_for_ipssl
    log "ipssl：将为公网 IP ${PUBLIC_IP} 申请 Let's Encrypt 短期证书（约 6 天，自动续期）"
  fi

  exec bash "${QOSNAT_INSTALL_DIR}/deploy-qos-nat.sh" start
}

main "$@"
