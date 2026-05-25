package ocserv

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ResolveSocketFile 解析 occtl 实际 Unix socket 路径。
// 启用 isolate-workers 时 ocserv 会在 socket-file 后追加哈希后缀（如 .950d4c7f.0）。
func ResolveSocketFile(configured string) (string, error) {
	base := strings.TrimSpace(configured)
	if base == "" {
		base = "/var/run/ocserv-socket"
	}
	if fi, err := os.Stat(base); err == nil && fi.Mode()&os.ModeSocket != 0 {
		return base, nil
	}
	matches, err := filepath.Glob(base + "*")
	if err != nil {
		return "", err
	}
	var sockets []string
	for _, m := range matches {
		fi, err := os.Stat(m)
		if err != nil || fi.Mode()&os.ModeSocket == 0 {
			continue
		}
		sockets = append(sockets, m)
	}
	if len(sockets) == 0 {
		return "", fmt.Errorf("ocserv socket 不存在（配置 %s；请确认 ocserv 已运行且已启用 use-occtl）", base)
	}
	// 多个 worker socket 时取主控 socket（通常后缀 .0）
	best := sockets[0]
	for _, s := range sockets {
		if strings.HasSuffix(s, ".0") {
			return s, nil
		}
		if len(s) < len(best) {
			best = s
		}
	}
	return best, nil
}
