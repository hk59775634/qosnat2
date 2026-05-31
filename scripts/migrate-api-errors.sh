#!/usr/bin/env bash
# 将 writeJSON(..., map[string]string{"error": ...}) 迁移为 writeAPIError 系列 helper。
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT/internal/api"

perl -i -0pe '
  s/writeJSON\(w, http\.StatusBadRequest, map\[string\]string\{"error": "bad json"\}\)/writeBadJSON(w)/g;
  s/writeJSON\(w, http\.StatusBadRequest, map\[string\]string\{"error": err\.Error\(\)\}\)/writeBadRequest(w, err.Error())/g;
  s/writeJSON\(w, http\.StatusBadRequest, map\[string\]string\{"error": (\$\w+)\}\)/writeBadRequest(w, $1)/g;
  s/writeJSON\(w, http\.StatusNotFound, map\[string\]string\{"error": ([^}]+)\}\)/writeNotFound(w, $1)/g;
  s/writeJSON\(w, http\.StatusForbidden, map\[string\]string\{"error": ([^}]+)\}\)/writeForbidden(w, "", $1)/g;
  s/writeJSON\(w, http\.StatusUnauthorized, map\[string\]string\{"error": ([^}]+)\}\)/writeUnauthorized(w, $1)/g;
  s/writeJSON\(w, http\.StatusConflict, map\[string\]string\{"error": err\.Error\(\)\}\)/writeConflict(w, err.Error())/g;
  s/writeJSON\(w, http\.StatusConflict, map\[string\]string\{"error": ([^}]+)\}\)/writeConflict(w, $1)/g;
  s/writeJSON\(w, http\.StatusServiceUnavailable, map\[string\]string\{"error": err\.Error\(\)\}\)/writeUnavailable(w, "", err.Error())/g;
  s/writeJSON\(w, http\.StatusServiceUnavailable, map\[string\]string\{"error": ([^}]+)\}\)/writeUnavailable(w, "", $1)/g;
  s/writeJSON\(w, http\.StatusInternalServerError, map\[string\]string\{"error": ([^}]+)\}\)/writeInternalError(w, $1)/g;
  s/writeJSON\(w, http\.StatusTooManyRequests, map\[string\]string\{"error": ([^}]+)\}\)/writeRateLimited(w, $1)/g;
' ./*.go

echo "Migrated error responses in internal/api/*.go"
