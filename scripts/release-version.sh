#!/usr/bin/env bash
# qosnat2 版本号：YYYYMMDD（8 位 UTC）+ 每日自增 2 位（01–99）。
# Manifest：releases/qosnat2-versions.json，仅保留最新 max_keep 条（默认 5）。
#
# 用法:
#   ./scripts/release-version.sh qosnat2 next
#   ./scripts/release-version.sh qosnat2 bump <id>
#   ./scripts/release-version.sh qosnat2 latest
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MAX_KEEP="${RELEASE_MAX_KEEP:-5}"

die() { echo "[release-version] ERROR: $*" >&2; exit 1; }
log() { echo "[release-version] $*" >&2; }

product="${1:-}"
cmd="${2:-}"
arg="${3:-}"

[[ "${product}" == "qosnat2" ]] || die "usage: $0 qosnat2 <next|bump|latest> [version]"
MANIFEST="${ROOT}/releases/qosnat2-versions.json"
TODAY="$(date -u +%Y%m%d)"

need_jq() {
  command -v jq >/dev/null || die "需要 jq（apt install jq）"
}

valid_qosnat_id() {
  local id="${1#v}"
  [[ "${#id}" -eq 10 && "${id}" =~ ^[0-9]{10}$ ]] || return 1
  [[ "${id:8:2}" != "00" && $((10#${id:8:2})) -le 99 ]]
}

max_seq_today() {
  need_jq
  [[ -f "${MANIFEST}" ]] || { echo 0; return; }
  jq -r --arg d "${TODAY}" '
    [.versions[]? | .id // "" |
     select(length == 10 and startswith($d)) | .[8:10] | tonumber] | max // 0
  ' "${MANIFEST}" 2>/dev/null || echo 0
}

cmd_next() {
  local seq=$(( $(max_seq_today) + 1 ))
  [[ "${seq}" -le 99 ]] || die "今日版本序号已达 99 上限"
  printf '%s%02d' "${TODAY}" "${seq}"
}

cmd_bump() {
  local id="${arg:-}"
  [[ -n "${id}" ]] || die "bump 需要 version_id"
  id="${id#v}"
  valid_qosnat_id "${id}" || die "非法版本号: ${id}（应为 YYYYMMDDNN）"
  need_jq
  mkdir -p "$(dirname "${MANIFEST}")"
  [[ -f "${MANIFEST}" ]] || echo '{"schema":1,"max_keep":5,"versions":[]}' > "${MANIFEST}"
  local now tmp
  now="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  tmp="$(mktemp)"
  jq --arg id "${id}" --arg tag "v${id}" --arg now "${now}" --argjson keep "${MAX_KEEP}" '
    .schema = 1 | .max_keep = $keep |
    .versions = ([{id: $id, tag: $tag, published_at: $now}]
      + ([.versions[]? | select(.id != $id)]) | .[0:$keep])
  ' "${MANIFEST}" > "${tmp}"
  mv "${tmp}" "${MANIFEST}"
  log "manifest 已更新: ${MANIFEST} (+${id})"
  echo "v${id}"
}

cmd_latest() {
  need_jq
  jq -r '.versions[0].tag // empty' "${MANIFEST}"
}

case "${cmd}" in
  next) cmd_next ;;
  bump) cmd_bump ;;
  latest) cmd_latest ;;
  *) die "unknown command: ${cmd}" ;;
esac
