#!/usr/bin/env bash
# qosnat2 版本号：YYYYMMDD（8 位 UTC）+ 每日自增 2 位（01–99）。
# ocserv 版本号：与上游官方 tag 一致（如 1.4.2），与 qosnat2 独立。
# Manifest：releases/{product}-versions.json，仅保留最新 max_keep 条（默认 5）。
#
# 用法:
#   ./scripts/release-version.sh qosnat2 next|bump|latest
#   ./scripts/release-version.sh ocserv bump <官方版本>   # 如 1.4.2
#   ./scripts/release-version.sh ocserv latest
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MAX_KEEP="${RELEASE_MAX_KEEP:-5}"

die() { echo "[release-version] ERROR: $*" >&2; exit 1; }
log() { echo "[release-version] $*" >&2; }

product="${1:-}"
cmd="${2:-}"
arg="${3:-}"

[[ -n "${product}" ]] || die "usage: $0 <qosnat2|ocserv> <cmd> [version]"
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

valid_qosnat_id() {
  local id="$1"
  [[ "${#id}" -eq 10 ]] || return 1
  [[ "${id}" =~ ^[0-9]{10}$ ]] || return 1
  local seq="${id:8:2}"
  [[ "${seq}" != "00" && $((10#${seq})) -le 99 ]] || return 1
  return 0
}

valid_ocserv_id() {
  local id="$1"
  [[ "${id}" =~ ^[0-9]+\.[0-9]+(\.[0-9]+)?([-.+][0-9A-Za-z.-]*)?$ ]]
}

normalize_ocserv_id() {
  local v="${1:-}"
  v="${v#ocserv-}"
  v="${v#v}"
  echo "${v}"
}

max_seq_today() {
  need_jq
  [[ -f "${MANIFEST}" ]] || { echo 0; return; }
  jq -r --arg d "${TODAY}" '
    [.versions[]? | .id // "" |
     select(length == 10 and startswith($d)) | .[8:10] | tonumber] | max // 0
  ' "${MANIFEST}" 2>/dev/null || echo 0
}

cmd_qosnat_next() {
  local seq max
  max="$(max_seq_today)"
  seq=$((max + 1))
  [[ "${seq}" -le 99 ]] || die "今日版本序号已达 99 上限"
  printf '%s%02d' "${TODAY}" "${seq}"
}

manifest_bump() {
  local id="$1"
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

cmd_bump() {
  local id="${arg:-}"
  [[ -n "${id}" ]] || die "bump 需要 version 参数"
  case "${product}" in
    qosnat2)
      id="${id#v}"
      valid_qosnat_id "${id}" || die "非法 qosnat2 版本号: ${id}（应为 YYYYMMDDNN）"
      ;;
    ocserv)
      id="$(normalize_ocserv_id "${id}")"
      valid_ocserv_id "${id}" || die "非法 ocserv 版本号: ${id}（应为官方版本如 1.4.2）"
      ;;
  esac
  manifest_bump "${id}"
}

cmd_latest() {
  need_jq
  [[ -f "${MANIFEST}" ]] || die "manifest 不存在: ${MANIFEST}"
  local tag
  tag="$(jq -r '.versions[0].tag // empty' "${MANIFEST}")"
  [[ -n "${tag}" ]] || die "manifest 中无可用版本"
  echo "${tag}"
}

case "${product}:${cmd}" in
  qosnat2:next)  cmd_qosnat_next ;;
  qosnat2:bump|ocserv:bump) cmd_bump ;;
  qosnat2:latest|ocserv:latest) cmd_latest ;;
  ocserv:next) die "ocserv 不使用日期版本号，请 bump 官方版本（如 1.4.2）" ;;
  *) die "unknown: ${product} ${cmd}" ;;
esac
