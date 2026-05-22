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
	ID     string   `json:"id"`
	Name   string   `json:"name,omitempty"`
	VNI    int      `json:"vni"`
	Local  string   `json:"local"`
	Remote string   `json:"remote"`
	Port   int      `json:"port,omitempty"`
	Underlay string `json:"underlay,omitempty"` // 可选 underlay 设备名
	IPv4   []string `json:"ipv4,omitempty"`
	Up     bool     `json:"up"`
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

// VXLANIfaceName 默认隧道接口名
func VXLANIfaceName(vni int) string {
	return fmt.Sprintf("vxlan%d", vni)
}
