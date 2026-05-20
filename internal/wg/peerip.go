package wg

import (
	"net"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

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
