// Package linknet 定义 qosnat2 内部链路地址（198.18.0.0/15），与 VPN 用户池 10.0.0.0/8 隔离。
package linknet

import "fmt"

const (
	// InternalCIDR RFC 2544 基准网段，仅供 WARP veth / ProxyEgress TUN 等内部链路。
	InternalCIDR = "198.18.0.0/15"

	// WARP veth 固定占用 198.18.0.0/30。
	WarpVethSubnet   = "198.18.0.0/30"
	WarpHostVethCIDR = "198.18.0.1/30"
	WarpNSVethCIDR   = "198.18.0.2/30"
	WarpNSVethGW     = "198.18.0.1"
	WarpWanGateway   = "198.18.0.2"

	// ProxyTunMax 与 store.ProxyEgressTunMax 一致。
	ProxyTunMax = 64
)

// ProxyTunHostCIDR 返回 ProxyEgress TUN 主机侧地址 198.18.(index+1).1/30（index 0..63；0.0/30 预留给 WARP）。
func ProxyTunHostCIDR(index int) string {
	if index < 0 || index >= ProxyTunMax {
		return ""
	}
	return fmt.Sprintf("198.18.%d.1/30", index+1)
}
