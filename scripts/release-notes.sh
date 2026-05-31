#!/usr/bin/env bash
# 发布更新说明：发布前编辑 release/pending-release-notes.md，CI 校验并归档至 releases/notes/<id>.md。
#
# 用法:
#   ./scripts/release-notes.sh draft          # 自上一 tag 生成待分类草稿
#   ./scripts/release-notes.sh validate       # 校验待发版说明（CI / 本地）
#   ./scripts/release-notes.sh finalize <id>  # 归档并输出 summary（单行，供 manifest）
#   ./scripts/release-notes.sh reset          # 重置待发版模板
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PENDING="${ROOT}/release/pending-release-notes.md"
NOTES_DIR="${ROOT}/releases/notes"
TEMPLATE="${ROOT}/release/pending-release-notes.template.md"

die() { echo "[release-notes] ERROR: $*" >&2; exit 1; }
log() { echo "[release-notes] $*" >&2; }

write_template() {
  mkdir -p "$(dirname "${PENDING}")"
  cp "${TEMPLATE}" "${PENDING}"
}

cmd_draft() {
  mkdir -p "$(dirname "${PENDING}")"
  [[ -f "${TEMPLATE}" ]] || die "missing template: ${TEMPLATE}"
  local last_tag commits
  last_tag="$("${ROOT}/scripts/release-version.sh" qosnat2 latest 2>/dev/null || true)"
  if [[ -z "${last_tag}" ]]; then
    last_tag="$(git -C "${ROOT}" tag -l 'v[0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9][0-9]' --sort=-version:refname | head -1 || true)"
  fi
  commits=""
  if [[ -n "${last_tag}" ]]; then
    commits="$(git -C "${ROOT}" log "${last_tag}..HEAD" --pretty=format:'- %s' --no-merges \
      | grep -Ev '^- chore\(release\):' || true)"
  else
    commits="$(git -C "${ROOT}" log -20 --pretty=format:'- %s' --no-merges \
      | grep -Ev '^- chore\(release\):' || true)"
  fi
  cp "${TEMPLATE}" "${PENDING}"
  {
    echo ""
    echo "## 待分类（自 ${last_tag:-首个版本} 以来的提交，请移至上方对应栏目后删除本节）"
    echo ""
    if [[ -n "${commits}" ]]; then
      echo "${commits}"
    else
      echo "- （无新提交或未找到上一版本 tag）"
    fi
  } >> "${PENDING}"
  log "草稿已写入 ${PENDING}"
}

is_placeholder_line() {
  local line="$1"
  line="$(echo "${line}" | sed 's/^[[:space:]-]*//;s/[[:space:]]*$//')"
  [[ -z "${line}" ]] && return 0
  [[ "${line}" == "（无）" ]] && return 0
  [[ "${line}" == "（一句话概括本版重点，将写入版本清单 summary 字段）" ]] && return 0
  return 1
}

extract_summary() {
  awk '
    /^## 概要/ { in_summary=1; next }
    /^## / { if (in_summary) exit; in_summary=0 }
    in_summary && NF {
      gsub(/^[[:space:]-]+/, "", $0)
      if ($0 != "" && $0 !~ /^（/) { print; exit }
    }
  ' "${PENDING}"
}

count_section_bullets() {
  local section="$1"
  awk -v sec="${section}" '
    $0 ~ "^## " sec { active=1; next }
    /^## / { if (active) exit; active=0 }
    active && /^- / {
      line=$0
      sub(/^- /, "", line)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", line)
      if (line != "" && line !~ /^（/) c++
    }
    END { print c+0 }
  ' "${PENDING}"
}

cmd_validate() {
  [[ -f "${PENDING}" ]] || die "缺少待发版说明: ${PENDING}（请先运行: ./scripts/release-notes.sh draft）"
  local summary total
  summary="$(extract_summary)"
  [[ -n "${summary}" ]] || die "「## 概要」不能为空（勿保留模板占位文字）"
  total=0
  for sec in 新增 优化 修复 删除 其他; do
    total=$(( total + $(count_section_bullets "${sec}") ))
  done
  [[ "${total}" -gt 0 ]] || die "至少在一个栏目（新增/优化/修复/删除/其他）下填写一条有效说明"
  log "校验通过: ${summary}"
}

cmd_finalize() {
  local id="${1:-}"
  [[ -n "${id}" ]] || die "finalize 需要 version_id"
  id="${id#v}"
  [[ "${#id}" -eq 10 ]] || die "非法版本号: ${id}"
  cmd_validate
  mkdir -p "${NOTES_DIR}"
  local out summary
  out="${NOTES_DIR}/${id}.md"
  {
    echo "# qosnat2 ${id}"
    echo ""
    sed '/^## 待分类/,$d' "${PENDING}" | sed '/^# 待发版更新说明$/d' | sed '/^> /d' | sed '/^$/N;/^\n$/D'
  } > "${out}"
  summary="$(extract_summary)"
  log "已归档: ${out}"
  write_template
  log "已重置 ${PENDING}"
  printf '%s' "${summary}"
}

cmd_reset() {
  write_template
  log "已重置 ${PENDING}"
}

cmd="${1:-}"
case "${cmd}" in
  draft) cmd_draft ;;
  validate) cmd_validate ;;
  finalize) cmd_finalize "${2:-}" ;;
  reset) cmd_reset ;;
  *)
    die "usage: $0 <draft|validate|finalize <id>|reset>"
    ;;
esac
