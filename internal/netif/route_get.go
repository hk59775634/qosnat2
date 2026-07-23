package netif

import (
	"encoding/json"
	"os/exec"
	"strings"
)

type routeGetRow struct {
	Gateway string `json:"gateway"`
	Dev     string `json:"dev"`
}

// RouteGetNexthop 用 `ip -json route get` 解析到目标的网关与出接口。
func RouteGetNexthop(dst string) (gateway, device string) {
	dst = strings.TrimSpace(dst)
	if dst == "" {
		return "", ""
	}
	out, err := exec.Command("ip", "-json", "route", "get", dst).Output()
	if err != nil {
		return "", ""
	}
	var rows []routeGetRow
	if err := json.Unmarshal(out, &rows); err != nil || len(rows) == 0 {
		return "", ""
	}
	dev := strings.TrimSpace(rows[0].Dev)
	if dev == "" || dev == "lo" {
		return "", ""
	}
	return strings.TrimSpace(rows[0].Gateway), dev
}
