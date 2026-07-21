// Package linknet 定义 qosnat2 内部与 VPN 隧道地址规划（198.18.0.0/15），避免与客户 LAN 的 10.0.0.0/8 冲突。
package linknet

import (
	"fmt"
	"net"
	"strings"
)

const (
	// InternalCIDR RFC 2544 基准网段，供 WARP / ProxyEgress / WireGuard / OCServ 等使用。
	InternalCIDR = "198.18.0.0/15"

	// WARP veth 固定占用 198.18.0.0/30。
	WarpVethSubnet   = "198.18.0.0/30"
	WarpHostVethCIDR = "198.18.0.1/30"
	WarpNSVethCIDR   = "198.18.0.2/30"
	WarpNSVethGW     = "198.18.0.1"
	WarpWanGateway   = "198.18.0.2"

	// ProxyTunMax 与 store.ProxyEgressTunMax 一致；占用 198.18.1.0/30 … 198.18.64.0/30。
	ProxyTunMax = 64

	// WireGuard 默认用户隧道占用 198.19.0.0/16（仍属 198.18.0.0/15）。
	// 实例 n 使用 198.19.n.0/24（n=0 为默认实例）。
	WireGuardDefaultAddress = "198.19.0.1/24"
	WireGuardMaxInstances   = 256

	// OCServ 默认用户池：198.18.250.0/24（避开 WARP 0.0/30、Proxy 1–64、WireGuard 198.19/16）。
	OCServDefaultIPv4Network = "198.18.250.0"
	OCServDefaultIPv4Netmask = "255.255.255.0"
	OCServDefaultIPv4CIDR    = "198.18.250.0/24"
)

// ProxyTunHostCIDR 返回 ProxyEgress TUN 主机侧地址 198.18.(index+1).1/30（index 0..63；0.0/30 预留给 WARP）。
func ProxyTunHostCIDR(index int) string {
	if index < 0 || index >= ProxyTunMax {
		return ""
	}
	return fmt.Sprintf("198.18.%d.1/30", index+1)
}

// WireGuardHostCIDR 返回 WireGuard 实例 index 的本端地址 198.19.{index}.1/24。
func WireGuardHostCIDR(index int) string {
	if index < 0 || index >= WireGuardMaxInstances {
		return WireGuardDefaultAddress
	}
	return fmt.Sprintf("198.19.%d.1/24", index)
}

// WireGuardSuggestPeerAllowedIP 按服务端隧道网段建议对端 /32（主机号 .10）。
func WireGuardSuggestPeerAllowedIP(serverCIDR string) string {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(serverCIDR))
	if err != nil || ip.To4() == nil || ipnet == nil {
		return "198.19.0.10/32"
	}
	base := ip.To4()
	ones, bits := ipnet.Mask.Size()
	if bits != 32 || ones > 24 {
		return base.String() + "/32"
	}
	suggest := make(net.IP, 4)
	copy(suggest, base)
	suggest[3] = 10
	if !ipnet.Contains(suggest) || suggest.Equal(base) {
		suggest[3] = 2
	}
	return suggest.String() + "/32"
}

// WireGuardClientFallbackAddr 在 peer 未提供 AllowedIPs 时，用服务端网段的 .2/32。
func WireGuardClientFallbackAddr(serverCIDR string) string {
	ip, ipnet, err := net.ParseCIDR(strings.TrimSpace(serverCIDR))
	if err != nil || ip.To4() == nil || ipnet == nil {
		return "198.19.0.2/32"
	}
	base := ip.To4()
	suggest := make(net.IP, 4)
	copy(suggest, base)
	suggest[3] = 2
	if suggest.Equal(base) {
		suggest[3] = 3
	}
	return suggest.String() + "/32"
}
