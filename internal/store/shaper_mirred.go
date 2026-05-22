package store

import (
	"net"
	"strings"
)

// CollectMirredCIDRs 返回需在 LAN ingress 安装 u32+mirred 的源网段（policy + 各 profile，去重）。
// 含 /32 单主机；按前缀长度从长到短排序由调用方处理。
func CollectMirredCIDRs(sh ShaperState) []string {
	seen := make(map[string]struct{})
	var out []string
	add := func(c string) {
		c = strings.TrimSpace(c)
		if c == "" {
			return
		}
		if _, _, err := net.ParseCIDR(c); err != nil {
			if ip := net.ParseIP(c); ip != nil && ip.To4() != nil {
				c = ip.String() + "/32"
			} else {
				return
			}
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	add(sh.PolicyCIDR)
	for _, p := range sh.Profiles {
		add(p.CIDR)
	}
	return out
}
