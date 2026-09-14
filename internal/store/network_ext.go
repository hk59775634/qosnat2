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
	ID           string `json:"id"`
	Name         string `json:"name"`
	Device       string `json:"device"`
	Gateway      string `json:"gateway"`
	Metric       int    `json:"metric"`
	Tier         int    `json:"tier"`
	Weight       int    `json:"weight"`
	PolicyOnly   bool   `json:"policy_only,omitempty"` // true: 不参与 main default，仅用于策略路由
	Enabled      bool   `json:"enabled"`
	WarpManaged  bool   `json:"warp_managed,omitempty"`  // true: 由 WARP 连接自动创建，不可手动删除
	ProxyManaged bool   `json:"proxy_managed,omitempty"` // true: 由 ProxyEgress/sing-box 自动创建，不可手动删除
	IfaceManaged bool   `json:"iface_managed,omitempty"` // true: 由接口页策略路由自动创建，不可手动改删

	// 网关健康探测（failover）：不影响 Enabled 用户意图，仅运行时排除路由。
	MonitorEnabled       bool   `json:"monitor_enabled,omitempty"`
	MonitorAddr          string `json:"monitor_addr,omitempty"`           // 探测目标，空则用 Gateway
	MonitorIntervalSec   int    `json:"monitor_interval_sec,omitempty"`   // 默认 5
	MonitorLossThreshold int    `json:"monitor_loss_threshold,omitempty"` // 连续失败次数，默认 3
}

// IfaceConfig 由 qosnat 写入 netplan 的物理网卡（/etc/netplan/99-qosnat2.yaml）
type IfaceConfig struct {
	Device string   `json:"device"`
	IPv4   []string `json:"ipv4,omitempty"`
	Up     bool     `json:"up"`
	DHCP4  bool     `json:"dhcp4,omitempty"`
	// Gateway 该口默认网关（主机 IPv4）。不写入 netplan default，避免抢主 WAN。
	Gateway string `json:"gateway,omitempty"`
	// PolicyRouting 为 true 时，为该口托管 IPv4 安装源地址策略路由（经 Gateway 回程）。
	PolicyRouting bool `json:"policy_routing,omitempty"`
	// LfnEnabled 长肥链路：该口队列改 fq、转发 MSS clamp；任一接口开启则整机 BBR + 大 TCP 窗口。
	LfnEnabled bool `json:"lfn_enabled,omitempty"`
	// LfnMssClamp TCP MSS；0 且开关开时用默认 1280；开关关则不钳。
	LfnMssClamp int `json:"lfn_mss_clamp,omitempty"`
}

// NetworkState VLAN / VXLAN / 多 WAN / netplan 托管接口 / 虚拟 IP
type NetworkState struct {
	Ifaces         []IfaceConfig  `json:"ifaces"`
	VLANs          []VLANIface    `json:"vlans"`
	VXLANTunnels   []VXLANTunnel  `json:"vxlan_tunnels"`
	WanLinks       []WanLink      `json:"wan_links"`
	EgressPolicies []EgressPolicy `json:"egress_policies,omitempty"`
	ProxyEgress    []ProxyEgress  `json:"proxy_egress,omitempty"` // HTTP/HTTPS/SOCKS5 独立 IP 出口
	VirtualIPs     []VirtualIP    `json:"virtual_ips,omitempty"`  // NAT/转发用 IP Alias
	WarpEnabled    bool           `json:"warp_enabled,omitempty"`
	WarpLicenseKey string         `json:"warp_license_key,omitempty"` // 持久化；由 WARP status API 返回明文供管理页确认
}

// FindIfaceConfig 按设备名查找托管网卡配置。
func FindIfaceConfig(st State, device string) (IfaceConfig, bool) {
	device = strings.TrimSpace(device)
	for _, ic := range st.Network.Ifaces {
		if ic.Device == device {
			return ic, true
		}
	}
	return IfaceConfig{}, false
}

// UpsertIfaceConfig 按设备名更新或追加托管网卡配置。
// gateway / policyRouting / lfn* 为 nil 时保留原值；传入空字符串可清空 gateway。
func UpsertIfaceConfig(st *State, device string, ipv4 []string, up *bool, dhcp4 *bool, gateway *string, policyRouting *bool, lfnEnabled *bool, lfnMssClamp *int) {
	device = strings.TrimSpace(device)
	if device == "" {
		return
	}
	for i := range st.Network.Ifaces {
		if st.Network.Ifaces[i].Device == device {
			applyIfaceConfigPatch(&st.Network.Ifaces[i], ipv4, up, dhcp4, gateway, policyRouting, lfnEnabled, lfnMssClamp)
			return
		}
	}
	entry := IfaceConfig{Device: device, Up: true}
	applyIfaceConfigPatch(&entry, ipv4, up, dhcp4, gateway, policyRouting, lfnEnabled, lfnMssClamp)
	st.Network.Ifaces = append(st.Network.Ifaces, entry)
}

func applyIfaceConfigPatch(ic *IfaceConfig, ipv4 []string, up *bool, dhcp4 *bool, gateway *string, policyRouting *bool, lfnEnabled *bool, lfnMssClamp *int) {
	if ipv4 != nil {
		ic.IPv4 = append([]string(nil), ipv4...)
	}
	if up != nil {
		ic.Up = *up
	}
	if dhcp4 != nil {
		ic.DHCP4 = *dhcp4
	}
	if gateway != nil {
		ic.Gateway = strings.TrimSpace(*gateway)
	}
	if policyRouting != nil {
		ic.PolicyRouting = *policyRouting
	}
	if lfnEnabled != nil {
		ic.LfnEnabled = *lfnEnabled
	}
	if lfnMssClamp != nil {
		ic.LfnMssClamp = *lfnMssClamp
	}
}

// RemoveIfaceConfig 停止由 qosnat2/netplan 托管该物理网卡（从 state.ifaces 移除）。
func RemoveIfaceConfig(st *State, device string) bool {
	device = strings.TrimSpace(device)
	if device == "" || st == nil {
		return false
	}
	keep := make([]IfaceConfig, 0, len(st.Network.Ifaces))
	removed := false
	for _, ic := range st.Network.Ifaces {
		if ic.Device == device {
			removed = true
			continue
		}
		keep = append(keep, ic)
	}
	st.Network.Ifaces = keep
	return removed
}

// AnyLFNEnabled 是否有接口开启长肥链路（整机 BBR 由此推导）。
func AnyLFNEnabled(st State) bool {
	for _, ic := range st.Network.Ifaces {
		if ic.LfnEnabled {
			return true
		}
	}
	return false
}

const (
	DefaultLFNMSS = 1280
	MinLFNMSS     = 536
	MaxLFNMSS     = 9000
	LFNTxQueueLen = 10000
)

// ValidateLFNMSSClamp 校验接口 MSS；0 表示跟随开关使用默认值。
func ValidateLFNMSSClamp(n int) error {
	if n == 0 {
		return nil
	}
	if n < MinLFNMSS || n > MaxLFNMSS {
		return fmt.Errorf("lfn_mss_clamp must be 0 or %d–%d", MinLFNMSS, MaxLFNMSS)
	}
	return nil
}

// EffectiveLFNMSS 开关开启时的钳制值；mtu>40 时不超过 MTU-40。
func EffectiveLFNMSS(enabled bool, clamp, mtu int) int {
	if !enabled {
		return 0
	}
	mss := clamp
	if mss <= 0 {
		mss = DefaultLFNMSS
	}
	if mtu > 40 {
		max := mtu - 40
		if mss > max {
			mss = max
		}
	}
	if mss < MinLFNMSS {
		mss = MinLFNMSS
	}
	return mss
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
	w.MonitorAddr = strings.TrimSpace(w.MonitorAddr)
	if w.MonitorIntervalSec < 0 {
		w.MonitorIntervalSec = 0
	}
	if w.MonitorIntervalSec > 3600 {
		w.MonitorIntervalSec = 3600
	}
	if w.MonitorLossThreshold < 0 {
		w.MonitorLossThreshold = 0
	}
	if w.MonitorLossThreshold > 100 {
		w.MonitorLossThreshold = 100
	}
	return nil
}
