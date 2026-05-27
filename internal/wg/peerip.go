package wg

import (
	"net"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// tunnelIPv4FromAddress 从实例 Address（CIDR 或裸 IP）取 IPv4 主机地址
func tunnelIPv4FromAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if !strings.Contains(addr, "/") {
		if ip := net.ParseIP(addr); ip != nil && ip.To4() != nil {
			return ip.String()
		}
		return ""
	}
	ip, _, err := net.ParseCIDR(addr)
	if err != nil || ip.To4() == nil {
		return ""
	}
	return ip.String()
}

// PeerRateShapeIP 用于限速与 eBPF 主机键：服务端仍按对端 AllowedIPs；客户端对端常为 0.0.0.0/0，
// 应使用本机隧道地址（Address），否则无法命中 host_exact / HTB。
func PeerRateShapeIP(inst store.WireGuardInstance, p store.WGPeer) string {
	if inst.Mode == store.WGModeClient {
		if ip := tunnelIPv4FromAddress(inst.Address); ip != "" {
			return ip
		}
	}
	return PeerTunnelIP(p)
}

// PeerTunnelIP 从 AllowedIPs 取第一个 IPv4 主机地址（用于 /32 限速）
func PeerTunnelIP(p store.WGPeer) string {
	for _, cidr := range p.AllowedIPs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if !strings.Contains(cidr, "/") {
			if ip := net.ParseIP(cidr); ip != nil && ip.To4() != nil {
				return ip.String()
			}
			continue
		}
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil || ip.To4() == nil {
			continue
		}
		return ip.String()
	}
	return ""
}
