package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// VLANIface 802.1Q 子接口
type VLANIface struct {
	ID     string   `json:"id"`
	Parent string   `json:"parent"`
	VID    int      `json:"vid"`
	Name   string   `json:"name,omitempty"`
	IPv4   []string `json:"ipv4,omitempty"`
	Up     bool     `json:"up"`
}

// WanLink 多 WAN 网关（Tier 越小越优先，Metric 用于 ip route）
type WanLink struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Device  string `json:"device"`
	Gateway string `json:"gateway"`
	Metric  int    `json:"metric"`
	Tier    int    `json:"tier"`
	Weight  int    `json:"weight"`
	Enabled bool   `json:"enabled"`
}

// IfaceConfig 由 qosnat 写入 netplan 的物理网卡（/etc/netplan/99-qosnat2.yaml）
type IfaceConfig struct {
	Device string   `json:"device"`
	IPv4   []string `json:"ipv4,omitempty"`
	Up     bool     `json:"up"`
	DHCP4  bool     `json:"dhcp4,omitempty"`
}

// NetworkState VLAN / VXLAN / 多 WAN / netplan 托管接口
type NetworkState struct {
	Ifaces          []IfaceConfig   `json:"ifaces"`
	VLANs           []VLANIface     `json:"vlans"`
	VXLANTunnels    []VXLANTunnel   `json:"vxlan_tunnels"`
	WanLinks        []WanLink       `json:"wan_links"`
	EgressPolicies  []EgressPolicy  `json:"egress_policies,omitempty"`
}

// UpsertIfaceConfig 按设备名更新或追加托管网卡配置
func UpsertIfaceConfig(st *State, device string, ipv4 []string, up *bool, dhcp4 *bool) {
	device = strings.TrimSpace(device)
	if device == "" {
		return
	}
	for i := range st.Network.Ifaces {
		if st.Network.Ifaces[i].Device == device {
			if ipv4 != nil {
				st.Network.Ifaces[i].IPv4 = append([]string(nil), ipv4...)
			}
			if up != nil {
				st.Network.Ifaces[i].Up = *up
			}
			if dhcp4 != nil {
				st.Network.Ifaces[i].DHCP4 = *dhcp4
			}
			return
		}
	}
	entry := IfaceConfig{Device: device, Up: true}
	if ipv4 != nil {
		entry.IPv4 = append([]string(nil), ipv4...)
	}
	if up != nil {
		entry.Up = *up
	}
	if dhcp4 != nil {
		entry.DHCP4 = *dhcp4
	}
	st.Network.Ifaces = append(st.Network.Ifaces, entry)
}

func NewVLANID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "vlan-" + hex.EncodeToString(b[:])
}

func NewWanLinkID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "wan-" + hex.EncodeToString(b[:])
}

// NormalizeWanLink 校验多 WAN 项
func NormalizeWanLink(w *WanLink) error {
	if w == nil {
		return fmt.Errorf("wan link nil")
	}
	if w.ID == "" {
		w.ID = NewWanLinkID()
	}
	w.Name = strings.TrimSpace(w.Name)
	w.Device = strings.TrimSpace(w.Device)
	w.Gateway = strings.TrimSpace(w.Gateway)
	if w.Device == "" {
		return fmt.Errorf("device required")
	}
	if w.Metric <= 0 {
		w.Metric = 100 + w.Tier*10
	}
	return nil
}
