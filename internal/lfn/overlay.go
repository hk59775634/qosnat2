package lfn

import "strings"

// Sysctl LFN 开启时整机覆盖（现场 1G@~150ms 验证：64MB 窗口 + BBR + fq）。
var Sysctl = map[string]string{
	"net.ipv4.tcp_congestion_control":    "bbr",
	"net.core.default_qdisc":             "fq",
	"net.ipv4.tcp_rmem":                  "4096 262144 67108864",
	"net.ipv4.tcp_wmem":                  "4096 65536 67108864",
	"net.ipv4.tcp_mtu_probing":           "1",
	"net.ipv4.tcp_slow_start_after_idle": "0",
}

// Overlay 在任一接口开启长肥链路时覆盖 extra；关闭时原样返回（由 catalog Defaults 恢复 cubic/fq_codel）。
func Overlay(extra map[string]string, enabled bool) map[string]string {
	out := cloneMap(extra)
	if !enabled {
		return out
	}
	for k, v := range Sysctl {
		out[k] = v
	}
	return out
}

func cloneMap(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		out[k] = v
	}
	return out
}

// LiveProtected 已有 clsact/HTB 时禁止 replace 根 qdisc。
func LiveProtected(qdiscShow string) bool {
	s := strings.ToLower(qdiscShow)
	return strings.Contains(s, "clsact") || strings.Contains(s, "htb")
}

// DeviceProtected 整形附加口或现场已有 clsact/HTB。
func DeviceProtected(dev string, shaperDevs []string, qdiscShow string) bool {
	dev = strings.TrimSpace(dev)
	for _, d := range shaperDevs {
		if strings.TrimSpace(d) == dev {
			return true
		}
	}
	return LiveProtected(qdiscShow)
}
