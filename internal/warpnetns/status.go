package warpnetns

import "strings"

// WarpStatusConnected 解析 warp-cli status 输出（兼容多版本文案）。
func WarpStatusConnected(raw string) bool {
	low := strings.ToLower(strings.TrimSpace(raw))
	if low == "" {
		return false
	}
	if strings.Contains(low, "unable to connect") || strings.Contains(low, "no network") {
		return false
	}
	if strings.Contains(low, "disconnected") {
		return false
	}
	if strings.Contains(low, "status update: connected") {
		return true
	}
	if strings.Contains(low, "invalid argument") || strings.Contains(low, "no such file") {
		return false
	}
	return strings.Contains(low, "connected")
}
