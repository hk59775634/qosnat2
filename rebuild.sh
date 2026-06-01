#!/usr/bin/env bash
# 本机快速重建：前端 + BPF + qosnatd，并重启 systemd 服务。
#
# 用法:
#   sudo ./rebuild.sh              # 开发构建（读 WEB_ROOT 下 web/dist，与 deploy 默认一致）
#   sudo RELEASE=1 ./rebuild.sh    # release 构建（内嵌 web/dist + BPF，与 GitHub Release 相同）
#
# 环境变量:
#   QOSNATD_BIN      安装路径（默认 /usr/local/bin/qosnatd）
#   BPF_INSTALL_DIR  BPF 安装目录（默认 /usr/lib/qosnat2）
#   RELEASE=1        使用 scripts/build-release.sh 产物并安装 dist/qosnatd-linux-amd64
#   SKIP_WEB=1       跳过 npm build（仅当 web/dist 已存在时）
#   SKIP_BPF=1       跳过 BPF 编译
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
QOSNATD_BIN="${QOSNATD_BIN:-/usr/local/bin/qosnatd}"
BPF_INSTALL_DIR="${BPF_INSTALL_DIR:-/usr/lib/qosnat2}"
ENV_FILE="${ENV_FILE:-/etc/qosnat2/env}"

log()  { echo "[$(date '+%F %T')] $*"; }
warn() { echo "[$(date '+%F %T')] WARN: $*" >&2; }
die()  { echo "[$(date '+%F %T')] ERROR: $*" >&2; exit 1; }

[[ "$(id -u)" -eq 0 ]] || die "请使用 root 运行: sudo $0"

admin_port() {
  local p="${ADMIN_PORT:-}"
  if [[ -z "${p}" && -f "${ENV_FILE}" ]]; then
    p="$(grep -E '^ADMIN_PORT=' "${ENV_FILE}" 2>/dev/null | head -1 | cut -d= -f2- | tr -d ' \"')"
  fi
  if [[ -z "${p}" ]]; then
    p="8080"
  fi
  echo "${p}"
}

health_check() {
  local port try_https=0 health_ok=0
  port="$(admin_port)"
  if [[ -f "${ENV_FILE}" ]] && grep -q '^TLS_CERT=' "${ENV_FILE}" 2>/dev/null; then
    try_https=1
  fi
  for _ in $(seq 1 20); do
    if [[ "${try_https}" -eq 1 ]]; then
      if curl -skf "https://127.0.0.1:${port}/api/v1/health" >/dev/null 2>&1; then
        log "health OK: https://127.0.0.1:${port}/api/v1/health"
        health_ok=1
        break
      fi
    fi
    if curl -sf "http://127.0.0.1:${port}/api/v1/health" >/dev/null 2>&1; then
      log "health OK: http://127.0.0.1:${port}/api/v1/health"
      health_ok=1
      break
    fi
    sleep 1
  done
  [[ "${health_ok}" -eq 1 ]] || warn "health 检查未通过（端口 ${port}；请确认 ${ENV_FILE} 中 ADMIN_PORT / TLS）"
}

cd "${ROOT}"

if [[ "${RELEASE:-0}" == "1" ]]; then
  log "RELEASE=1：执行 scripts/build-release.sh（内嵌 UI + BPF）"
  if [[ "${SKIP_WEB:-0}" != "1" ]]; then
    if command -v npm &>/dev/null && [[ -f "${ROOT}/web/package.json" ]]; then
      log "构建 Web UI"
      (cd "${ROOT}/web" && (npm ci --silent 2>/dev/null || npm install --silent) && npm run build)
    else
      die "未找到 npm，无法构建 web/dist"
    fi
  else
    [[ -f "${ROOT}/web/dist/index.html" ]] || die "SKIP_WEB=1 但 ${ROOT}/web/dist/index.html 不存在"
  fi
  if [[ "${SKIP_BPF:-0}" != "1" ]]; then
    if command -v clang &>/dev/null && [[ -f "${ROOT}/bpf/Makefile" ]]; then
      log "编译 BPF"
      (cd "${ROOT}/bpf" && make clean all)
    else
      warn "跳过 BPF 编译"
    fi
  fi
  chmod +x "${ROOT}/scripts/build-release.sh"
  (cd "${ROOT}" && ./scripts/build-release.sh)
  install -m 0755 "${ROOT}/dist/qosnatd-linux-amd64" "${QOSNATD_BIN}"
  if [[ -f "${ROOT}/dist/lib/classify.bpf.o" ]]; then
    install -d "${BPF_INSTALL_DIR}"
    install -m 0644 "${ROOT}/dist/lib/classify.bpf.o" "${BPF_INSTALL_DIR}/classify.bpf.o"
  fi
else
  log "开发构建（未内嵌 UI；需 WEB_ROOT 指向 web/dist，见 ${ENV_FILE}）"
  go mod tidy

  if [[ "${SKIP_BPF:-0}" != "1" ]] && command -v clang &>/dev/null && [[ -f "${ROOT}/bpf/Makefile" ]]; then
    log "编译并安装 BPF -> ${BPF_INSTALL_DIR}"
    (cd "${ROOT}/bpf" && make clean all && make install INSTALL_DIR="${BPF_INSTALL_DIR}")
  elif [[ "${SKIP_BPF:-0}" == "1" ]]; then
    warn "SKIP_BPF=1：跳过 BPF"
  else
    warn "跳过 BPF 编译（无 clang 或 bpf/Makefile）"
  fi

  if [[ "${SKIP_WEB:-0}" != "1" ]]; then
    if command -v npm &>/dev/null && [[ -f "${ROOT}/web/package.json" ]]; then
      log "构建 Web UI -> ${ROOT}/web/dist"
      (cd "${ROOT}/web" && (npm ci --silent 2>/dev/null || npm install --silent) && npm run build)
    else
      die "未找到 npm，无法构建 web/dist"
    fi
  else
    [[ -f "${ROOT}/web/dist/index.html" ]] || die "SKIP_WEB=1 但 web/dist 不存在"
    warn "SKIP_WEB=1：使用已有 web/dist"
  fi

  log "编译 qosnatd（dev）-> ${QOSNATD_BIN}"
  go build -o "${ROOT}/bin/qosnatd" ./cmd/qosnatd
  install -m 0755 "${ROOT}/bin/qosnatd" "${QOSNATD_BIN}"
fi

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

health_check

if [[ "${RELEASE:-0}" == "1" ]]; then
  log "完成。release 二进制: ${QOSNATD_BIN}（内嵌 UI）"
else
  log "完成。Web: ${ROOT}/web/dist  BPF: ${BPF_INSTALL_DIR}/classify.bpf.o"
  log "请确认 ${ENV_FILE} 中 WEB_ROOT=${ROOT}/web/dist（或 dist 子目录）"
fi
