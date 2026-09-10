package conntrack

import (
	"os/exec"
	"strings"
)

const maxFlushCIDRs = 64

// FlushByCIDRs 按源/目的 CIDR 删除 conntrack 条目，使新 SNAT/策略路由立即作用于新会话。
// 未安装 conntrack 工具时静默跳过。
func FlushByCIDRs(cidrs []string) {
	if len(cidrs) == 0 {
		return
	}
	if _, err := exec.LookPath("conntrack"); err != nil {
		return
	}
	seen := map[string]struct{}{}
	n := 0
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" || c == "0.0.0.0/0" || c == "::/0" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		_ = exec.Command("conntrack", "-D", "-s", c).Run()
		_ = exec.Command("conntrack", "-D", "-d", c).Run()
		n++
		if n >= maxFlushCIDRs {
			return
		}
	}
}
