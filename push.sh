#!/usr/bin/env bash
# 推送 main 并触发 republish-release，覆盖指定 tag 的 GitHub Release 资产。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "${ROOT}"

usage() {
  cat <<'EOF'
用法:
  ./push.sh <version_tag>

示例:
  ./push.sh v2026053005
  ./push.sh 2026053005          # 可省略 v 前缀

说明:
  1. git push origin main
  2. gh workflow run republish-release.yml -f version_tag=<tag>

前提: 已 gh auth login，且 republish-release.yml 已存在于 origin/main。
EOF
}

die() { echo "push.sh: $*" >&2; exit 1; }

TAG="${1:-}"
if [[ -z "${TAG}" ]]; then
  usage
  exit 1
fi

TAG="v${TAG#v}"
if [[ ! "${TAG}" =~ ^v[0-9]{10}$ ]]; then
  die "非法版本号: ${TAG}（应为 vYYYYMMDDNN，如 v2026053005）"
fi

command -v gh >/dev/null || die "需要 gh CLI（请先 gh auth login）"
gh auth status >/dev/null 2>&1 || die "gh 未登录，请执行: gh auth login"

git push origin main
gh workflow run republish-release.yml -f "version_tag=${TAG}"
echo "已推送 main，并已触发 republish-release（${TAG}）"
