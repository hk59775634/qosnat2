#!/usr/bin/env bash
# qosnat2 一键安装（类似 aaPanel：curl | bash）
#
# 说明：一键安装功能仅在 Ubuntu 24.04 上完成安装验证，强烈推荐使用 Ubuntu 24.04。
#
# 默认 HTTP 安装（建议带防缓存参数，避免 CDN/代理返回旧脚本）：
#   curl -fsSL -H 'Cache-Control: no-cache' \
#     "https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh?t=$(date +%s)" | bash
#
# 启用 HTTPS（公网 IP + Let's Encrypt 短期 IP 证书，HTTP-01 需 80 端口可达）：
#   curl -fsSL -H 'Cache-Control: no-cache' \
#     "https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh?t=$(date +%s)" | bash -s -- ipssl
#
# 可选环境变量：
#   ACME_EMAIL=...               Let's Encrypt 账户邮箱（ipssl；未设置时默认 hk59775634@gmail.com）
#   PUBLIC_IP=1.2.3.4            指定公网 IPv4（默认自动探测）
#   ACME_STAGING=1               使用 LE 测试环境
#   QOSNAT_SKIP_OS_CHECK=1       非 Ubuntu 24.04 时仍继续（不推荐）
#   QOSNAT_INSTALL_DIR=/opt/qosnat2
#   QOSNAT_REPO=https://github.com/hk59775634/qosnat2.git
#   QOSNAT_RELEASE_TAG=v2026052801  指定版本（10 位 YYYYMMDDNN 或带 v 前缀）
#   QOSNAT_VERSIONS_URL=...      版本清单（默认 GitHub releases/qosnat2-versions.json）
#   QOSNAT_SKIP_SCRIPT_REFRESH=1  跳过从 GitHub 重新拉取 install.sh（仅本地调试）
#
# 卸载见 scripts/uninstall.sh 或 deploy-qos-nat.sh uninstall

set -euo pipefail

QOSNAT_REPO="${QOSNAT_REPO:-https://github.com/hk59775634/qosnat2.git}"
QOSNAT_INSTALL_DIR="${QOSNAT_INSTALL_DIR:-/opt/qosnat2}"
QOSNAT_INSTALL_RAW_URL="${QOSNAT_INSTALL_RAW_URL:-https://raw.githubusercontent.com/hk59775634/qosnat2/main/scripts/install.sh}"
QOSNAT_RELEASE_TAG="${QOSNAT_RELEASE_TAG:-}"
QOSNAT_VERSIONS_URL="${QOSNAT_VERSIONS_URL:-https://raw.githubusercontent.com/hk59775634/qosnat2/main/releases/qosnat2-versions.json}"
QOSNAT_RELEASE_ASSET="${QOSNAT_RELEASE_ASSET:-qosnat2-linux-amd64.tar.gz}"
QOSNAT_BIN_PATH="${QOSNAT_BIN_PATH:-/usr/local/bin/qosnatd}"
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

# GitHub 直连超时/失败时依次尝试 gh-proxy 加速（中国等地区）。
GH_PROXY_MIRRORS=(
  "https://v4.gh-proxy.org/"
  "https://cdn.gh-proxy.org/"
)

curl_github_url() {
  local url="$1"
  local out="${2:--}"
  local proxy url_try curl_out=()
  if [[ "${out}" != "-" ]]; then
    curl_out=(-o "${out}")
  fi
  # 国内优先 v4.gh-proxy.org，直连 GitHub 作最后备选。
  for proxy in "${GH_PROXY_MIRRORS[@]}"; do
    url_try="${proxy}${url}"
    if curl -fsSL --connect-timeout 8 --max-time 300 --retry 2 --retry-delay 1 \
        "${curl_out[@]}" "${url_try}"; then
      return 0
    fi
    warn "加速失败，尝试下一镜像: ${url_try}"
  done
  warn "gh-proxy 均失败，尝试直连 GitHub: ${url}"
  if curl -fsSL --connect-timeout 8 --max-time 120 --retry 2 --retry-delay 1 \
      "${curl_out[@]}" "${url}"; then
    return 0
  fi
  return 1
}

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
  log "安装运行时必要软件包（apt）…"
  if ! DEBIAN_FRONTEND=noninteractive apt-get update -qq; then
    die "apt-get update 失败"
  fi
  # release 安装仅保留运行时依赖（目标机不编译）
  if ! DEBIAN_FRONTEND=noninteractive apt-get install -y -qq \
    ca-certificates curl git gnupg \
    iproute2 nftables \
    wireguard-tools tcpdump conntrack dnsmasq \
    tar; then
    die "必要软件包安装失败，请检查 apt 源与网络后重试"
  fi
  # apt dnsmasq 默认 enable+start；qosnat2 默认未启用 DHCP/DNS
  systemctl disable --now dnsmasq >/dev/null 2>&1 || true
  log "必要软件包已安装"
  need_cmd git
  need_cmd curl
  need_cmd tar
}

need_cmd() {
  command -v "$1" &>/dev/null || die "安装后仍缺少命令: $1"
}

# curl | bash 时 $0 常为 bash；从磁盘直接执行 install.sh 则跳过自更新。
should_refresh_install_script() {
  [[ "${QOSNAT_SKIP_SCRIPT_REFRESH:-0}" == "1" ]] && return 1
  [[ -n "${QOSNAT_INSTALL_REFRESHED:-}" ]] && return 1
  local src="${BASH_SOURCE[0]:-}"
  [[ -f "${src}" ]] && [[ "${src}" == */install.sh ]] && return 1
  return 0
}

# 规避 raw.githubusercontent.com / 中间代理缓存，确保管道安装拿到最新 install.sh
bootstrap_refresh_install_script() {
  should_refresh_install_script || return 0
  local tmp
  tmp="$(mktemp /tmp/qosnat2-install.XXXXXX.sh)"
  if ! curl_github_url "${QOSNAT_INSTALL_RAW_URL}?t=$(date +%s)" "${tmp}" 2>/dev/null; then
    rm -f "${tmp}"
    warn "无法从 GitHub 拉取最新 install.sh，将使用当前脚本继续"
    return 0
  fi
  chmod 700 "${tmp}"
  log "已拉取最新 install.sh，继续安装…"
  export QOSNAT_INSTALL_REFRESHED=1
  exec env QOSNAT_INSTALL_REFRESHED=1 bash "${tmp}" "$@"
}

clone_or_update() {
  local ref="${QOSNAT_RELEASE_TAG:-main}"
  if [[ -d "${QOSNAT_INSTALL_DIR}/.git" ]]; then
    log "同步仓库到 ${ref}…"
    # --force：本地 tag 与远端不一致时避免 "would clobber existing tag" 导致 fetch 失败
    git -C "${QOSNAT_INSTALL_DIR}" fetch --tags --force origin \
      || die "git fetch 失败（请检查网络，或删除 ${QOSNAT_INSTALL_DIR} 后重试）"
    if [[ "${ref}" == "main" ]]; then
      git -C "${QOSNAT_INSTALL_DIR}" reset --hard "origin/main" || die "git reset 失败"
    else
      git -C "${QOSNAT_INSTALL_DIR}" checkout -f "${ref}" || die "git checkout ${ref} 失败"
    fi
    local rev
    rev="$(git -C "${QOSNAT_INSTALL_DIR}" rev-parse --short HEAD 2>/dev/null || echo unknown)"
    log "当前代码版本: ${rev}"
  elif [[ -e "${QOSNAT_INSTALL_DIR}" ]]; then
    die "${QOSNAT_INSTALL_DIR} 已存在且不是 git 仓库"
  else
    log "克隆 ${QOSNAT_REPO} -> ${QOSNAT_INSTALL_DIR}"
    git clone --depth 1 "${QOSNAT_REPO}" "${QOSNAT_INSTALL_DIR}"
    if [[ "${ref}" != "main" ]]; then
      git -C "${QOSNAT_INSTALL_DIR}" fetch --depth 1 origin "refs/tags/${ref}:refs/tags/${ref}" \
        || die "拉取 tag ${ref} 失败"
      git -C "${QOSNAT_INSTALL_DIR}" checkout -f "${ref}" || die "checkout ${ref} 失败"
    fi
  fi
}

normalize_qosnat_tag() {
  local t="${1:-}"
  t="${t#v}"
  if [[ "${t}" =~ ^[0-9]{10}$ ]]; then
    echo "v${t}"
    return 0
  fi
  echo "${1}"
}

detect_release_tag() {
  if [[ -n "${QOSNAT_RELEASE_TAG}" ]]; then
    normalize_qosnat_tag "${QOSNAT_RELEASE_TAG}"
    return 0
  fi
  local tag json
  if command -v jq >/dev/null 2>&1; then
    json="$(curl_github_url "${QOSNAT_VERSIONS_URL}" || die "无法拉取版本清单: ${QOSNAT_VERSIONS_URL}")"
    tag="$(echo "${json}" | jq -r '.versions[0].tag // empty')"
  else
    json="$(curl_github_url "${QOSNAT_VERSIONS_URL}" || die "无法拉取版本清单: ${QOSNAT_VERSIONS_URL}")"
    tag="$(echo "${json}" | python3 -c "import json,sys; d=json.load(sys.stdin); v=d.get('versions') or []; print(v[0].get('tag','') if v else '')" 2>/dev/null || true)"
  fi
  [[ -n "${tag}" ]] || die "无法从版本清单获取 release tag，请设置 QOSNAT_RELEASE_TAG 或检查 ${QOSNAT_VERSIONS_URL}"
  normalize_qosnat_tag "${tag}"
}

download_release_binary() {
  local tag url tmp tgz
  tag="$(detect_release_tag)"
  QOSNAT_RELEASE_TAG="${tag}"
  url="https://github.com/hk59775634/qosnat2/releases/download/${tag}/${QOSNAT_RELEASE_ASSET}"
  tmp="$(mktemp -d /tmp/qosnat2-release.XXXXXX)"
  tgz="${tmp}/${QOSNAT_RELEASE_ASSET}"
  log "下载 release: ${url}"
  curl_github_url "${url}" "${tgz}" || die "下载 release 失败（已尝试 gh-proxy）: ${url}"
  tar -xzf "${tgz}" -C "${tmp}" || die "解压 release 包失败"
  [[ -f "${tmp}/qosnatd-linux-amd64" ]] || die "release 包缺少 qosnatd-linux-amd64"
  install -m 0755 "${tmp}/qosnatd-linux-amd64" "${QOSNAT_BIN_PATH}"
  if [[ -f "${tmp}/lib/classify.bpf.o" ]]; then
    install -d /usr/lib/qosnat2
    install -m 0644 "${tmp}/lib/classify.bpf.o" /usr/lib/qosnat2/classify.bpf.o
  fi
  if [[ -f "${tmp}/lib/dnsmasq-chnroutes" ]]; then
    install -d /usr/local/lib/qosnat2
    # staging+rename，避免 ETXTBSY
    install -m 0755 "${tmp}/lib/dnsmasq-chnroutes" /usr/local/lib/qosnat2/dnsmasq-chnroutes.new
    mv -f /usr/local/lib/qosnat2/dnsmasq-chnroutes.new /usr/local/lib/qosnat2/dnsmasq-chnroutes
    if command -v dnsmasq &>/dev/null && ! dnsmasq --help 2>&1 | grep -q chnroutes-file; then
      if systemctl is-active --quiet dnsmasq 2>/dev/null; then
        systemctl stop dnsmasq || true
      fi
      if [[ -x /usr/sbin/dnsmasq && ! -f /usr/sbin/dnsmasq.dist ]]; then
        cp -a /usr/sbin/dnsmasq /usr/sbin/dnsmasq.dist
      fi
      install -m 0755 "${tmp}/lib/dnsmasq-chnroutes" /usr/sbin/dnsmasq.new
      if [[ -e /usr/sbin/dnsmasq ]]; then
        mv -f /usr/sbin/dnsmasq /usr/sbin/dnsmasq.old
      fi
      mv -f /usr/sbin/dnsmasq.new /usr/sbin/dnsmasq
      rm -f /usr/sbin/dnsmasq.old
      # 默认不启动；由 qosnat2 DHCP Apply / 启动回放决定
      systemctl disable dnsmasq >/dev/null 2>&1 || true
      log "已从 release 安装 dnsmasq-chnroutes（服务保持停止，直至 DHCP/DNS 启用）"
    fi
  fi
  rm -rf "${tmp}"
  install -d /etc/qosnat2
  local id="${QOSNAT_RELEASE_TAG#v}"
  echo "${id}" > /etc/qosnat2/release-tag
  log "已安装 release 二进制: ${QOSNAT_BIN_PATH} (version=${id})"
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
  bootstrap_refresh_install_script "$@"
  download_release_binary
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

  export SKIP_BUILD=1
  exec bash "${QOSNAT_INSTALL_DIR}/deploy-qos-nat.sh" -SkipBuild start
}

main "$@"
