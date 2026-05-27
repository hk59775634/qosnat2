#!/usr/bin/env bash
# 开发机快速重建：前端 + BPF + qosnatd，并重启在线服务
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QOSNATD_BIN="${QOSNATD_BIN:-/usr/local/bin/qosnatd}"
BPF_INSTALL_DIR="${BPF_INSTALL_DIR:-/usr/lib/qosnat2}"

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "请使用 root 运行: sudo $0"

cd "${ROOT}"

log "go mod tidy"
go mod tidy

if command -v clang &>/dev/null && [[ -f "${ROOT}/bpf/Makefile" ]]; then
  log "编译并安装 BPF (classify.bpf.o)"
  (cd "${ROOT}/bpf" && make clean all && make install INSTALL_DIR="${BPF_INSTALL_DIR}")
else
  warn "跳过 BPF 编译（无 clang 或 bpf/Makefile）"
fi

if command -v npm &>/dev/null && [[ -f "${ROOT}/web/package.json" ]]; then
  log "构建 Web UI"
  (cd "${ROOT}/web" && (npm ci --silent 2>/dev/null || npm install --silent) && npm run build)
else
  die "未找到 npm，无法构建 web/dist"
fi

log "编译 qosnatd -> ${QOSNATD_BIN}"
go build -o "${ROOT}/bin/qosnatd" ./cmd/qosnatd
install -m 0755 "${ROOT}/bin/qosnatd" "${QOSNATD_BIN}"

if systemctl is-enabled qosnatd &>/dev/null; then
  log "重启 qosnatd"
  systemctl restart qosnatd
  sleep 2
  if systemctl is-active --quiet qosnatd; then
    log "qosnatd active"
  else
    systemctl status qosnatd --no-pager || true
    die "qosnatd 未处于 active 状态"
  fi
else
  warn "未安装 qosnatd systemd 单元，请手动启动 ${QOSNATD_BIN}"
fi

# 健康检查（优先 HTTPS，最多重试 10s）
health_ok=0
for _ in 1 2 3 4 5; do
  for url in "https://127.0.0.1:8080/api/v1/health" "http://127.0.0.1:8080/api/v1/health"; do
    if curl -skf "${url}" >/dev/null 2>&1; then
      log "health OK: ${url}"
      health_ok=1
      break 2
    fi
  done
  sleep 1
done
[[ "${health_ok}" -eq 1 ]] || warn "health 检查未通过（服务可能仍绑定其他端口）"

log "完成。Web: ${ROOT}/web/dist  BPF: ${BPF_INSTALL_DIR}/classify.bpf.o"
