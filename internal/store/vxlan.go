package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// VXLANTunnel L2 VXLAN overlay（netplan tunnels）
type VXLANTunnel struct {
	ID       string   `json:"id"`
	Name     string   `json:"name,omitempty"`
	VNI      int      `json:"vni"`
	Local    string   `json:"local"`
	Remote   string   `json:"remote"`
	Port     int      `json:"port,omitempty"`
	Underlay string   `json:"underlay,omitempty"` // 可选 underlay 设备名
	IPv4     []string `json:"ipv4,omitempty"`
	Up       bool     `json:"up"`
}

func NewVXLANID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "vxlan-" + hex.EncodeToString(b[:])
}

// NormalizeVXLANTunnel 校验 VXLAN 隧道参数
func NormalizeVXLANTunnel(v *VXLANTunnel) error {
	if v == nil {
		return fmt.Errorf("vxlan nil")
	}
	if v.ID == "" {
		v.ID = NewVXLANID()
	}
	if v.VNI < 1 || v.VNI > 16777215 {
		return fmt.Errorf("vni must be 1-16777215")
	}
	if v.Name == "" {
		v.Name = VXLANIfaceName(v.VNI)
	}
	loc := strings.TrimSpace(v.Local)
	rem := strings.TrimSpace(v.Remote)
	if loc == "" || rem == "" {
		return fmt.Errorf("local and remote underlay IP required")
	}
	if net.ParseIP(loc) == nil || net.ParseIP(rem) == nil {
		return fmt.Errorf("invalid local/remote ip")
	}
	v.Local, v.Remote = loc, rem
	if v.Port <= 0 {
		v.Port = 4789
	}
	v.Underlay = strings.TrimSpace(v.Underlay)
	return nil
}

// VXLANDefaultMTU 覆盖 1500 underlay 时的隧道 MTU（UDP+VXLAN+以太网约 50 字节）。
const VXLANDefaultMTU = 1450

// VXLANAutoEndpoint 已启用隧道的防火墙自动放行参数。
type VXLANAutoEndpoint struct {
	ID       string
	Iface    string
	Port     int
	Remote   string
	Underlay string
}

// CollectVXLANAutoEndpoints 收集处于 up 的 VXLAN，供 WAN UDP 与 LAN↔隧道 forward 自动规则。
func CollectVXLANAutoEndpoints(tunnels []VXLANTunnel) []VXLANAutoEndpoint {
	var out []VXLANAutoEndpoint
	for _, t := range tunnels {
		if !t.Up {
			continue
		}
		id := strings.TrimSpace(t.ID)
		if id == "" {
			continue
		}
		remote := strings.TrimSpace(t.Remote)
		if net.ParseIP(remote) == nil {
			continue
		}
		port := t.Port
		if port <= 0 {
			port = 4789
		}
		name := strings.TrimSpace(t.Name)
		if name == "" {
			if t.VNI < 1 {
				continue
			}
			name = VXLANIfaceName(t.VNI)
		}
		out = append(out, VXLANAutoEndpoint{
			ID:       id,
			Iface:    name,
			Port:     port,
			Remote:   remote,
			Underlay: strings.TrimSpace(t.Underlay),
		})
	}
	return out
}

// VXLANRemoteMatch 将对端 VTEP 写成 nft 源地址匹配（IPv4 /32，IPv6 /128）。
func VXLANRemoteMatch(remote string) (srcAddr, ipVersion string) {
	ip := net.ParseIP(strings.TrimSpace(remote))
	if ip == nil {
		return "", ""
	}
	if v4 := ip.To4(); v4 != nil {
		return v4.String() + "/32", ""
	}
	return ip.String() + "/128", "ipv6"
}

// VXLANIfaceName 默认隧道接口名
func VXLANIfaceName(vni int) string {
	return fmt.Sprintf("vxlan%d", vni)
}
