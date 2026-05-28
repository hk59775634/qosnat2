#!/usr/bin/env bash
# 在具备编译工具链的主机上构建 ocserv 预编译包（供 GitHub Release 与目标机一键安装）。
# 用法: ./scripts/build-ocserv-release.sh
# 产出: dist/ocserv-linux-amd64.tar.gz
# 发布: 创建 tag ocserv-<version> 并上传该 tar.gz 为 release 资产。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="${ROOT}/dist"
VERSION="${OCSERV_TAG:-1.4.2}"
VERSION="${VERSION#v}"
VERSION="${VERSION#ocserv-}"
TARBALL="${OUT_DIR}/ocserv-linux-amd64.tar.gz"
STAGE="$(mktemp -d /tmp/ocserv-pkg.XXXXXX)"

log() { echo "[build-ocserv-release] $*"; }

cleanup() { rm -rf "${STAGE}"; }
trap cleanup EXIT

[[ "$(id -u)" -eq 0 ]] || { echo "需要 root 执行编译安装阶段"; exit 1; }

log "编译安装 ocserv ${VERSION} 到临时前缀 ${STAGE}/prefix ..."
OCSERV_TAG="${VERSION}" \
  OCSERV_PREFIX="${STAGE}/prefix" \
  OCSERV_SYSCONFDIR="${STAGE}/etc/ocserv" \
  bash "${ROOT}/scripts/install-ocserv.sh" --method source

mkdir -p "${STAGE}/pkg/bin" "${STAGE}/pkg/systemd"
install -m 0755 "${STAGE}/prefix/sbin/ocserv" "${STAGE}/pkg/bin/ocserv"
for tool in occtl ocpasswd; do
  if [[ -x "${STAGE}/prefix/bin/${tool}" ]]; then
    install -m 0755 "${STAGE}/prefix/bin/${tool}" "${STAGE}/pkg/bin/${tool}"
  elif [[ -x "${STAGE}/prefix/sbin/${tool}" ]]; then
    install -m 0755 "${STAGE}/prefix/sbin/${tool}" "${STAGE}/pkg/bin/${tool}"
  fi
done
if [[ -f /etc/systemd/system/ocserv.service ]]; then
  cp /etc/systemd/system/ocserv.service "${STAGE}/pkg/systemd/ocserv.service"
fi
echo "${VERSION}" > "${STAGE}/pkg/VERSION"

mkdir -p "${OUT_DIR}"
tar -C "${STAGE}/pkg" -czf "${TARBALL}" .
log "完成: ${TARBALL}"
log "发布示例: gh release create ocserv-${VERSION} ${TARBALL} --title \"ocserv ${VERSION}\""
