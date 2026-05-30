#!/usr/bin/env bash
# 重新上传已有 GitHub Release 的资产（不 bump 版本清单）。
# 用法: ./scripts/republish-release-assets.sh v2026053005
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TAG="${1:-}"
[[ -n "${TAG}" ]] || { echo "usage: $0 vYYYYMMDDNN" >&2; exit 1; }
TAG="v${TAG#v}"

cd "${ROOT}"
chmod +x ./scripts/build-release.sh
./scripts/build-release.sh

command -v gh >/dev/null || { echo "需要 gh CLI 且已 gh auth login" >&2; exit 1; }
gh release upload "${TAG}" \
  dist/qosnatd-linux-amd64 \
  dist/qosnat2-linux-amd64.tar.gz \
  --clobber
echo "已更新 ${TAG} 资产"
