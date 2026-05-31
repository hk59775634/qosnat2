package api

import (
	"net/http"
	"strings"
)

func queryDryRun(r *http.Request) bool {
	v := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("dry_run")))
	return v == "1" || v == "true" || v == "yes"
}

// writeAPIError 返回兼容 envelope：保留 error 字段供现有前端使用，并附带 code。
func writeAPIError(w http.ResponseWriter, httpStatus int, code, message string) {
	writeJSON(w, httpStatus, map[string]any{
		"ok":    false,
		"error": message,
		"code":  code,
	})
}

func writeApplyError(w http.ResponseWriter, err error) {
	writeAPIError(w, http.StatusInternalServerError, "APPLY_FAILED", err.Error())
}

func writeDryRunOK(w http.ResponseWriter, extra map[string]any) {
	out := map[string]any{"ok": true, "dry_run": true, "nft_valid": true}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, http.StatusOK, out)
}
