package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// VirtualIPType 虚拟 IP 类型（v1 仅 IP Alias，专用于 NAT/端口转发绑定）。
const VirtualIPTypeIPAlias = "ip_alias"

// VirtualIP 附加在接口上的虚拟地址（与接口「主配置」分离，供共享 SNAT / 1:1 / 端口转发使用）。
type VirtualIP struct {
	ID        string `json:"id"`
	Type      string `json:"type"`      // ip_alias
	Interface string `json:"interface"` // 绑定网卡
	Address   string `json:"address"`   // 主机 IP 或 CIDR；主机默认 /32
	Comment   string `json:"comment,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// NewVirtualIPID 生成 vip- 前缀 ID。
func NewVirtualIPID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "vip-" + hex.EncodeToString(b[:])
}

// NormalizeVirtualIPAddress 校验并规范化地址，返回 CIDR 与主机 IP。
func NormalizeVirtualIPAddress(addr string) (cidr, host string, err error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "", "", fmt.Errorf("address required")
	}
	if strings.Contains(addr, "/") {
		ip, n, e := net.ParseCIDR(addr)
		if e != nil {
			return "", "", fmt.Errorf("invalid cidr %q", addr)
		}
		if ip.To4() == nil {
			return "", "", fmt.Errorf("only ipv4 virtual ips supported")
		}
		ones, bits := n.Mask.Size()
		if bits != 32 {
			return "", "", fmt.Errorf("only ipv4 virtual ips supported")
		}
		// Alias 建议 /32；若用户给了更短前缀仍接受（网段映射场景少见）。
		if ones < 8 || ones > 32 {
			return "", "", fmt.Errorf("invalid prefix length /%d", ones)
		}
		host = ip.To4().String()
		cidr = fmt.Sprintf("%s/%d", host, ones)
		return cidr, host, nil
	}
	ip := net.ParseIP(addr)
	if ip == nil || ip.To4() == nil {
		return "", "", fmt.Errorf("invalid ipv4 address %q", addr)
	}
	host = ip.To4().String()
	return host + "/32", host, nil
}

// NormalizeVirtualIP 校验并规范化一条虚拟 IP。
func NormalizeVirtualIP(v *VirtualIP) error {
	if v == nil {
		return fmt.Errorf("virtual ip nil")
	}
	if v.ID == "" {
		v.ID = NewVirtualIPID()
	}
	v.Type = strings.ToLower(strings.TrimSpace(v.Type))
	if v.Type == "" {
		v.Type = VirtualIPTypeIPAlias
	}
	if v.Type != VirtualIPTypeIPAlias {
		return fmt.Errorf("unsupported virtual ip type %q (only ip_alias)", v.Type)
	}
	v.Interface = strings.TrimSpace(v.Interface)
	if v.Interface == "" {
		return fmt.Errorf("interface required")
	}
	cidr, _, err := NormalizeVirtualIPAddress(v.Address)
	if err != nil {
		return err
	}
	v.Address = cidr
	v.Comment = strings.TrimSpace(v.Comment)
	return nil
}

// FindVirtualIP 按 ID 查找。
func FindVirtualIP(list []VirtualIP, id string) (VirtualIP, bool) {
	id = strings.TrimSpace(id)
	for _, v := range list {
		if v.ID == id {
			return v, true
		}
	}
	return VirtualIP{}, false
}

// VirtualIPHost 返回规范化后的主机 IP（不含前缀）。
func VirtualIPHost(v VirtualIP) string {
	_, host, err := NormalizeVirtualIPAddress(v.Address)
	if err != nil {
		return strings.TrimSpace(strings.Split(v.Address, "/")[0])
	}
	return host
}

// EnabledVirtualIPs 返回启用的虚拟 IP 副本。
func EnabledVirtualIPs(list []VirtualIP) []VirtualIP {
	var out []VirtualIP
	for _, v := range list {
		if v.Enabled {
			out = append(out, v)
		}
	}
	return out
}
