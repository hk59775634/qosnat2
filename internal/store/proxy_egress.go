package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/linknet"
)

const (
	// ProxyEgressTunPrefix TUN 接口名前缀（qpe0..qpe63）。
	ProxyEgressTunPrefix = "qpe"
	// ProxyEgressTunMax 最大并行代理出口数。
	ProxyEgressTunMax = linknet.ProxyTunMax
	// ProxyEgressWanLinkPrefix 托管 WanLink ID 前缀。
	ProxyEgressWanLinkPrefix = "wan-proxy-"
)

// ProxyEgress 第三方 HTTP/HTTPS/SOCKS5 代理出口（经 sing-box TUN 接入策略路由）。
type ProxyEgress struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // socks5 | http | https
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	// TunIndex 稳定分配的 TUN 序号（0..63）；-1 表示未分配。
	TunIndex int `json:"tun_index"`
	// Enabled 意图启用（断线后由 watchdog 重连）。
	Enabled bool `json:"enabled"`
	// EgressIP 最近一次探测到的出口公网 IP（展示用，可空）。
	EgressIP string `json:"egress_ip,omitempty"`
	// 出口探测详情（与 WARP exit_info 对齐）。
	EgressCountry   string `json:"egress_country,omitempty"`
	EgressCity      string `json:"egress_city,omitempty"`
	EgressRegion    string `json:"egress_region,omitempty"`
	EgressOrg       string `json:"egress_org,omitempty"`
	EgressCheckedAt string `json:"egress_checked_at,omitempty"`
	LastTestError   string `json:"last_test_error,omitempty"`
}

// NewProxyEgressID 生成 pe-<12hex> ID。
func NewProxyEgressID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "pe-" + hex.EncodeToString(b[:])
}

// ProxyWanLinkID 返回托管 WanLink ID。
func ProxyWanLinkID(proxyID string) string {
	return ProxyEgressWanLinkPrefix + strings.TrimSpace(proxyID)
}

// ProxyTunDevice 返回 TUN 设备名。
func ProxyTunDevice(tunIndex int) string {
	if tunIndex < 0 {
		return ""
	}
	return fmt.Sprintf("%s%d", ProxyEgressTunPrefix, tunIndex)
}

// ProxyTunAddress 返回 sing-box TUN inet4 地址（主机侧 /30，198.18.0.0/15 内部池）。
func ProxyTunAddress(tunIndex int) string {
	return linknet.ProxyTunHostCIDR(tunIndex)
}

// IsProxyWanLink 是否为 ProxyEgress 托管链路。
func IsProxyWanLink(w WanLink) bool {
	return w.ProxyManaged || strings.HasPrefix(w.ID, ProxyEgressWanLinkPrefix)
}

// IsManagedWanLink WARP 或 ProxyEgress 自动托管，不可手动改删。
func IsManagedWanLink(w WanLink) bool {
	return IsWarpWanLink(w) || IsProxyWanLink(w)
}

// ProxyEgressByID 按 ID 查找。
func ProxyEgressByID(list []ProxyEgress, id string) (ProxyEgress, bool) {
	id = strings.TrimSpace(id)
	for _, p := range list {
		if p.ID == id {
			return p, true
		}
	}
	return ProxyEgress{}, false
}

// NormalizeProxyEgress 校验并补全代理出口配置。
func NormalizeProxyEgress(p *ProxyEgress) error {
	if p == nil {
		return fmt.Errorf("proxy egress nil")
	}
	if p.ID == "" {
		p.ID = NewProxyEgressID()
	}
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		p.Name = p.ID
	}
	p.Type = strings.ToLower(strings.TrimSpace(p.Type))
	switch p.Type {
	case "socks5", "socks", "http", "https":
		if p.Type == "socks" {
			p.Type = "socks5"
		}
	default:
		return fmt.Errorf("type must be socks5, http, or https")
	}
	p.Server = strings.TrimSpace(p.Server)
	if p.Server == "" {
		return fmt.Errorf("server required")
	}
	if ip := net.ParseIP(p.Server); ip == nil {
		// allow hostname
		if strings.ContainsAny(p.Server, " /:\\") {
			return fmt.Errorf("invalid server host")
		}
	}
	if p.Port < 1 || p.Port > 65535 {
		return fmt.Errorf("port must be 1-65535")
	}
	p.Username = strings.TrimSpace(p.Username)
	p.Password = strings.TrimSpace(p.Password)
	p.EgressIP = strings.TrimSpace(p.EgressIP)
	if p.TunIndex < -1 {
		p.TunIndex = -1
	}
	if p.TunIndex >= ProxyEgressTunMax {
		return fmt.Errorf("tun_index out of range")
	}
	return nil
}

// AllocateProxyTunIndex 为新条目分配空闲 TUN 序号。
func AllocateProxyTunIndex(existing []ProxyEgress) (int, error) {
	used := map[int]struct{}{}
	for _, p := range existing {
		if p.TunIndex >= 0 {
			used[p.TunIndex] = struct{}{}
		}
	}
	for i := 0; i < ProxyEgressTunMax; i++ {
		if _, ok := used[i]; !ok {
			return i, nil
		}
	}
	return -1, fmt.Errorf("no free TUN index (max %d)", ProxyEgressTunMax)
}

// ProxyWanLink 构造 ProxyEgress 托管 WanLink（policy_only，无 gateway，依赖 device 直连路由）。
func ProxyWanLink(p ProxyEgress) WanLink {
	dev := ProxyTunDevice(p.TunIndex)
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = "Proxy " + p.ID
	}
	return WanLink{
		ID:           ProxyWanLinkID(p.ID),
		Name:         name,
		Device:       dev,
		Gateway:      "",
		Metric:       260,
		Tier:         8,
		Weight:       1,
		PolicyOnly:   true,
		Enabled:      p.Enabled,
		ProxyManaged: true,
	}
}

// UpsertProxyWanLink 写入或更新对应托管 WanLink。
func UpsertProxyWanLink(st *State, p ProxyEgress) {
	link := ProxyWanLink(p)
	id := link.ID
	for i, w := range st.Network.WanLinks {
		if w.ID == id || (w.ProxyManaged && w.Device == link.Device && link.Device != "") {
			st.Network.WanLinks[i] = link
			return
		}
	}
	st.Network.WanLinks = append(st.Network.WanLinks, link)
}

// RemoveProxyWanLink 移除指定代理的托管 WanLink（保留引用该链路的出站策略）。
func RemoveProxyWanLink(st *State, proxyID string) {
	wanID := ProxyWanLinkID(proxyID)
	var links []WanLink
	for _, w := range st.Network.WanLinks {
		if w.ID == wanID || (w.ProxyManaged && strings.HasSuffix(w.ID, proxyID)) {
			continue
		}
		links = append(links, w)
	}
	st.Network.WanLinks = links
}

// SyncProxyWanLinks 按 ProxyEgress 列表同步全部托管 WanLink（移除孤儿）。
func SyncProxyWanLinks(st *State) {
	want := map[string]ProxyEgress{}
	for _, p := range st.Network.ProxyEgress {
		if p.TunIndex < 0 {
			continue
		}
		want[ProxyWanLinkID(p.ID)] = p
	}
	var keep []WanLink
	for _, w := range st.Network.WanLinks {
		if !IsProxyWanLink(w) {
			keep = append(keep, w)
			continue
		}
		if p, ok := want[w.ID]; ok {
			keep = append(keep, ProxyWanLink(p))
			delete(want, w.ID)
		}
	}
	for _, p := range want {
		keep = append(keep, ProxyWanLink(p))
	}
	st.Network.WanLinks = keep
}

// SetProxyEgressEnabled 更新启用意图。
func SetProxyEgressEnabled(st *State, id string, enabled bool) bool {
	for i := range st.Network.ProxyEgress {
		if st.Network.ProxyEgress[i].ID == id {
			st.Network.ProxyEgress[i].Enabled = enabled
			return true
		}
	}
	return false
}

// SetProxyEgressExitInfo 更新探测到的出口信息。
func SetProxyEgressExitInfo(st *State, id string, info ProxyExitInfo) bool {
	for i := range st.Network.ProxyEgress {
		if st.Network.ProxyEgress[i].ID != id {
			continue
		}
		st.Network.ProxyEgress[i].EgressIP = strings.TrimSpace(info.IP)
		st.Network.ProxyEgress[i].EgressCountry = strings.TrimSpace(info.Country)
		st.Network.ProxyEgress[i].EgressCity = strings.TrimSpace(info.City)
		st.Network.ProxyEgress[i].EgressRegion = strings.TrimSpace(info.Region)
		st.Network.ProxyEgress[i].EgressOrg = strings.TrimSpace(info.Org)
		st.Network.ProxyEgress[i].EgressCheckedAt = strings.TrimSpace(info.CheckedAt)
		st.Network.ProxyEgress[i].LastTestError = strings.TrimSpace(info.Error)
		return true
	}
	return false
}

// ClearProxyEgressExitInfo 清除出口探测结果。
func ClearProxyEgressExitInfo(st *State, id string) bool {
	return SetProxyEgressExitInfo(st, id, ProxyExitInfo{})
}

// ProxyExitInfo 持久化的代理出口探测结果。
type ProxyExitInfo struct {
	IP        string
	Country   string
	City      string
	Region    string
	Org       string
	CheckedAt string
	Error     string
}

// ProxyExitInfoFromStore 从 ProxyEgress 构造 API exit_info。
func ProxyExitInfoFromStore(p ProxyEgress) map[string]any {
	out := map[string]any{}
	if ip := strings.TrimSpace(p.EgressIP); ip != "" {
		out["ip"] = ip
	}
	if v := strings.TrimSpace(p.EgressCountry); v != "" {
		out["country"] = v
	}
	if v := strings.TrimSpace(p.EgressCity); v != "" {
		out["city"] = v
	}
	if v := strings.TrimSpace(p.EgressRegion); v != "" {
		out["region"] = v
	}
	if v := strings.TrimSpace(p.EgressOrg); v != "" {
		out["org"] = v
	}
	if v := strings.TrimSpace(p.EgressCheckedAt); v != "" {
		out["fetched_at"] = v
	}
	if v := strings.TrimSpace(p.LastTestError); v != "" {
		out["error"] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetProxyEgressIP 更新探测到的出口 IP。
func SetProxyEgressIP(st *State, id, ip string) bool {
	ip = strings.TrimSpace(ip)
	for i := range st.Network.ProxyEgress {
		if st.Network.ProxyEgress[i].ID == id {
			st.Network.ProxyEgress[i].EgressIP = ip
			return true
		}
	}
	return false
}

// ProxyEgressPublicView 返回不含密码的副本（列表 API 用）。
func ProxyEgressPublicView(p ProxyEgress) ProxyEgress {
	out := p
	if out.Password != "" {
		out.Password = "***"
	}
	return out
}

// ParseProxyServerHostPort 兼容 "host:port" 粘贴；优先使用独立 port 字段。
func ParseProxyServerHostPort(server string, port int) (host string, p int, err error) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", 0, fmt.Errorf("empty server")
	}
	if port > 0 {
		return server, port, nil
	}
	h, portStr, e := net.SplitHostPort(server)
	if e != nil {
		return "", 0, fmt.Errorf("port required")
	}
	p, e = strconv.Atoi(portStr)
	if e != nil || p < 1 || p > 65535 {
		return "", 0, fmt.Errorf("invalid port")
	}
	return h, p, nil
}
