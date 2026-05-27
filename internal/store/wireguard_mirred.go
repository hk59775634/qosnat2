package store

import (
	"net"
	"strings"
)

// WireGuardMirredSrcCIDRs 返回应在 WireGuard 接口 ingress 上做 mirred→ifb0 的源网段（隧道内地址）。
// 与 LAN 侧 CollectMirredCIDRs 分离：对端流量从 wg 口进入，不会走 LAN ingress。
func WireGuardMirredSrcCIDRs(w WireGuardState) []string {
	if !w.Enabled {
		return nil
	}
	a := strings.TrimSpace(w.Address)
	if a == "" {
		return nil
	}
	_, n, err := net.ParseCIDR(a)
	if err != nil {
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
			return []string{ip.String() + "/32"}
		}
		return nil
	}
	return []string{n.String()}
}
