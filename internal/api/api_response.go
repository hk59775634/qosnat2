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
	if code == "" {
		code = defaultErrorCode(httpStatus)
	}
	writeJSON(w, httpStatus, map[string]any{
		"ok":    false,
		"error": message,
		"code":  code,
	})
}

func defaultErrorCode(httpStatus int) string {
	switch httpStatus {
	case http.StatusBadRequest:
		return "VALID_BAD_REQUEST"
	case http.StatusUnauthorized:
		return "AUTH_UNAUTHORIZED"
	case http.StatusForbidden:
		return "AUTH_FORBIDDEN"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusConflict:
		return "CONFLICT"
	case http.StatusUnprocessableEntity:
		return "VALID_UNPROCESSABLE"
	case http.StatusTooManyRequests:
		return "RATE_LIMITED"
	case http.StatusServiceUnavailable:
		return "UNAVAILABLE"
	default:
		return "INTERNAL_ERROR"
	}
}

func writeBadJSON(w http.ResponseWriter) {
	writeAPIError(w, http.StatusBadRequest, "VALID_BAD_JSON", "bad json")
}

func writeBadRequest(w http.ResponseWriter, message string) {
	writeAPIError(w, http.StatusBadRequest, "VALID_BAD_REQUEST", message)
}

func writeNotFound(w http.ResponseWriter, message string) {
	writeAPIError(w, http.StatusNotFound, "NOT_FOUND", message)
}

func writeForbidden(w http.ResponseWriter, code, message string) {
	if code == "" {
		code = "AUTH_FORBIDDEN"
	}
	writeAPIError(w, http.StatusForbidden, code, message)
}

func writeUnauthorized(w http.ResponseWriter, message string) {
	writeAPIError(w, http.StatusUnauthorized, "AUTH_UNAUTHORIZED", message)
}

func writeConflict(w http.ResponseWriter, message string) {
	writeAPIError(w, http.StatusConflict, "CONFLICT", message)
}

func writeAPIErrorWithExtra(w http.ResponseWriter, httpStatus int, code, message string, extra map[string]any) {
	if code == "" {
		code = defaultErrorCode(httpStatus)
	}
	out := map[string]any{
		"ok":    false,
		"error": message,
		"code":  code,
	}
	for k, v := range extra {
		out[k] = v
	}
	writeJSON(w, httpStatus, out)
}

func writeConflictWithExtra(w http.ResponseWriter, message string, extra map[string]any) {
	writeAPIErrorWithExtra(w, http.StatusConflict, "CONFLICT", message, extra)
}

func writeUnavailable(w http.ResponseWriter, code, message string) {
	if code == "" {
		code = "UNAVAILABLE"
	}
	writeAPIError(w, http.StatusServiceUnavailable, code, message)
}

func writeInternalError(w http.ResponseWriter, message string) {
	writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", message)
}

func writeRateLimited(w http.ResponseWriter, message string) {
	writeAPIError(w, http.StatusTooManyRequests, "RATE_LIMITED", message)
}

func writeMethodNotAllowed(w http.ResponseWriter) {
	writeAPIError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
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
