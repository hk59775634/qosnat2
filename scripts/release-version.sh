#!/usr/bin/env bash
# 版本号：YYYYMMDD（8 位 UTC 日期）+ 每日自增 2 位序号（01–99，每日重置）。
# Manifest：releases/{product}-versions.json，仅保留最新 max_keep 条（默认 5）。
#
# 用法:
#   ./scripts/release-version.sh qosnat2 next          # 计算下一版本号（不写文件）
#   ./scripts/release-version.sh qosnat2 bump <id>     # 写入 manifest（CI 在 release 成功后）
#   ./scripts/release-version.sh qosnat2 latest        # 输出最新版本的 github tag
#   ./scripts/release-version.sh ocserv next|bump|latest
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MAX_KEEP="${RELEASE_MAX_KEEP:-5}"

die() { echo "[release-version] ERROR: $*" >&2; exit 1; }
log() { echo "[release-version] $*" >&2; }

product="${1:-}"
cmd="${2:-}"
arg="${3:-}"

[[ -n "${product}" ]] || die "usage: $0 <qosnat2|ocserv> <next|bump|latest> [version_id]"
[[ "${product}" == "qosnat2" || "${product}" == "ocserv" ]] || die "unknown product: ${product}"

MANIFEST="${ROOT}/releases/${product}-versions.json"
TODAY="$(date -u +%Y%m%d)"

github_tag_for() {
  local id="$1"
  case "${product}" in
    qosnat2) echo "v${id}" ;;
    ocserv)  echo "ocserv-${id}" ;;
  esac
}

need_jq() {
  command -v jq >/dev/null || die "需要 jq（apt install jq）"
}

valid_id() {
  local id="$1"
  [[ "${#id}" -eq 10 ]] || return 1
  [[ "${id}" =~ ^[0-9]{10}$ ]] || return 1
  [[ "${id:0:8}" =~ ^[0-9]{8}$ ]] || return 1
  local seq="${id:8:2}"
  [[ "${seq}" =~ ^[0-9]{2}$ ]] || return 1
  [[ "${seq}" != "00" ]] || return 1
  [[ $((10#${seq})) -le 99 ]] || return 1
  return 0
}

max_seq_today() {
  need_jq
  [[ -f "${MANIFEST}" ]] || { echo 0; return; }
  jq -r --arg d "${TODAY}" '
    [.versions[]? | .id // .tag // "" | gsub("^v"; "") | gsub("^ocserv-"; "") |
     select(length == 10 and startswith($d)) | .[8:10] | tonumber] | max // 0
  ' "${MANIFEST}" 2>/dev/null || echo 0
}

cmd_next() {
  local seq max
  max="$(max_seq_today)"
  seq=$((max + 1))
  [[ "${seq}" -le 99 ]] || die "今日版本序号已达 99 上限"
  printf '%s%02d' "${TODAY}" "${seq}"
}

cmd_bump() {
  local id="${arg:-}"
  [[ -n "${id}" ]] || die "bump 需要 version_id 参数"
  id="${id#v}"
  id="${id#ocserv-}"
  valid_id "${id}" || die "非法版本号: ${id}（应为 YYYYMMDDNN）"
  need_jq
  mkdir -p "$(dirname "${MANIFEST}")"
  [[ -f "${MANIFEST}" ]] || echo '{"schema":1,"max_keep":5,"versions":[]}' > "${MANIFEST}"
  local ghtag now tmp
  ghtag="$(github_tag_for "${id}")"
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  tmp="$(mktemp)"
  jq --arg id "${id}" --arg tag "${ghtag}" --arg now "${now}" --argjson keep "${MAX_KEEP}" '
    .schema = 1 |
    .max_keep = $keep |
    .versions = (
      [{id: $id, tag: $tag, published_at: $now}]
      + ([.versions[]? | select(.id != $id and .tag != $tag)])
      | .[0:$keep]
    )
  ' "${MANIFEST}" > "${tmp}"
  mv "${tmp}" "${MANIFEST}"
  log "manifest 已更新: ${MANIFEST} (+${id}, keep ${MAX_KEEP})"
  echo "${ghtag}"
}

cmd_latest() {
  need_jq
  [[ -f "${MANIFEST}" ]] || die "manifest 不存在: ${MANIFEST}"
  local tag
  tag="$(jq -r '.versions[0].tag // empty' "${MANIFEST}")"
  [[ -n "${tag}" ]] || die "manifest 中无可用版本"
  echo "${tag}"
}

case "${cmd}" in
  next)   cmd_next ;;
  bump)   cmd_bump ;;
  latest) cmd_latest ;;
  *)      die "unknown command: ${cmd} (use next|bump|latest)" ;;
esac
