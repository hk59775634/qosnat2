#!/usr/bin/env bash
# 对比 server.go 注册路径与 openapi.yaml 中的 path 条目（粗略一致性检查）。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

ROUTES=$(grep -oE 'HandleFunc\("/api/v1[^"]+' internal/api/server.go | sed 's/HandleFunc("//' | sort -u)
OPENAPI=$(grep -E '^  /api/v1' api/openapi.yaml | sed 's/:$//' | sed 's/{[^}]*}/:id/g' | sort -u)

echo "=== Routes in server.go ($(echo "$ROUTES" | wc -l)) ==="
missing=0
while IFS= read -r r; do
  [[ -z "$r" ]] && continue
  # openapi 可能用路径参数模板，做前缀/模糊匹配
  if ! echo "$OPENAPI" | grep -qF "$(echo "$r" | sed 's/{[^}]*}/:id/g')"; then
    if ! grep -qF "$r" api/openapi.yaml 2>/dev/null; then
      echo "  MISSING in openapi: $r"
      missing=$((missing + 1))
    fi
  fi
done <<< "$ROUTES"

if [[ "$missing" -eq 0 ]]; then
  echo "OK: no obvious route gaps (manual review still recommended)"
  exit 0
fi
echo "WARN: $missing route(s) not found in openapi.yaml"
exit 1
