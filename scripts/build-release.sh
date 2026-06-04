#!/usr/bin/env bash
# 在具备完整工具链的机器上构建「单文件」qosnatd（内嵌 Web + BPF），
# 目标平台：linux/amd64（Ubuntu 24.04 x86_64）。
# 部署机无需 npm / Node / 源码树中的 web/dist。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ASSETS="${ROOT}/internal/webassets"
OUT_DIR="${ROOT}/dist"
BINARY="${OUT_DIR}/qosnatd-linux-amd64"
GOOS="${GOOS:-linux}"
GOARCH="${GOARCH:-amd64}"

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

if [[ "${GOOS}/${GOARCH}" != "linux/amd64" ]]; then
  warn "非 linux/amd64 交叉编译未在此脚本验收，继续: GOOS=${GOOS} GOARCH=${GOARCH}"
fi

need() {
  command -v "$1" &>/dev/null || die "缺少命令: $1"
}

need go
need rsync

cd "${ROOT}"

# --- Web（仅构建机执行一次）---
if [[ ! -f "${ROOT}/web/dist/index.html" ]]; then
  need npm
  log "web/dist 不存在，执行 npm run build..."
  (cd "${ROOT}/web" && (npm ci --silent 2>/dev/null || npm install --silent) && npm run build)
fi

# --- BPF ---
if [[ ! -f "${ROOT}/bpf/classify.bpf.o" ]]; then
  need clang
  log "编译 classify.bpf.o..."
  (cd "${ROOT}/bpf" && make)
fi

# --- 同步到 embed 目录 ---
log "同步嵌入资源 -> internal/webassets/"
rm -rf "${ASSETS}/static"
mkdir -p "${ASSETS}/static"
rsync -a --delete "${ROOT}/web/dist/" "${ASSETS}/static/"
cp -f "${ROOT}/bpf/classify.bpf.o" "${ASSETS}/classify.bpf.o"

# --- Go release 二进制 ---
log "编译 release qosnatd (${GOOS}/${GOARCH})..."
mkdir -p "${OUT_DIR}"
(
  cd "${ROOT}"
  go mod tidy
  CGO_ENABLED=0 GOOS="${GOOS}" GOARCH="${GOARCH}" \
    go build -tags release -trimpath \
    -ldflags="-s -w" \
    -o "${BINARY}" ./cmd/qosnatd
)

# 可选：同时安装 BPF 到系统路径（release 已内嵌，仅作兼容）
install -d "${OUT_DIR}/lib"
cp -f "${ASSETS}/classify.bpf.o" "${OUT_DIR}/lib/classify.bpf.o"

# --- dnsmasq-chnroutes（Ubuntu 24.04 amd64 预编译，目标机免编译）---
BUILD_DNSMASQ="${BUILD_DNSMASQ:-1}"
if [[ "${BUILD_DNSMASQ}" == "1" ]]; then
  chmod +x "${ROOT}/scripts/build-dnsmasq-chnroutes.sh" "${ROOT}/scripts/install-dnsmasq-chnroutes-binary.sh"
  log "编译 dnsmasq-chnroutes -> ${OUT_DIR}/lib/dnsmasq-chnroutes …"
  OUTPUT="${OUT_DIR}/lib/dnsmasq-chnroutes" bash "${ROOT}/scripts/build-dnsmasq-chnroutes.sh"
else
  warn "BUILD_DNSMASQ=0：release 包不含预编译 dnsmasq-chnroutes"
fi

TARBALL="${OUT_DIR}/qosnat2-linux-amd64.tar.gz"
TAR_ITEMS=("$(basename "${BINARY}")" lib/classify.bpf.o)
if [[ -f "${OUT_DIR}/lib/dnsmasq-chnroutes" ]]; then
  TAR_ITEMS+=("lib/dnsmasq-chnroutes")
fi
tar -C "${OUT_DIR}" -czf "${TARBALL}" "${TAR_ITEMS[@]}"

log "完成:"
log "  二进制: ${BINARY}"
log "  压缩包: ${TARBALL}"
log ""
log "部署示例（目标机 Ubuntu 24.04 x86_64，仅需 root + 运行时依赖 nft/tc 等）:"
log "  sudo install -m 0755 ${BINARY} /usr/local/bin/qosnatd"
log "  sudo ./deploy-qos-nat.sh -SkipWeb start   # 或仅 systemctl restart qosnatd"
