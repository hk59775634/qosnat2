package store

import "strings"

// WanLinkIDWarp 由「启用 WARP」自动创建的多 WAN 链路 ID（不可手动删除）。
const WanLinkIDWarp = "wan-warp"

// WarpWanLink 构造 WARP 托管 WAN 链路（policy_only，不参与主表 default）。
func WarpWanLink(device string) WanLink {
	device = strings.TrimSpace(device)
	if device == "" {
		device = "qwp0"
	}
	gateway := ""
	if device == "qwp0" {
		gateway = "10.99.0.2"
	}
	return WanLink{
		ID:          WanLinkIDWarp,
		Name:        "Cloudflare WARP",
		Device:      device,
		Gateway:     gateway,
		Metric:      250,
		Tier:        9,
		Weight:      1,
		PolicyOnly:  true,
		Enabled:     true,
		WarpManaged: true,
	}
}

// IsWarpWanLink 是否为 WARP 自动托管链路。
func IsWarpWanLink(w WanLink) bool {
	return w.WarpManaged || w.ID == WanLinkIDWarp
}

// UpsertWarpWanLink 写入或更新 WARP WAN 链路。
func UpsertWarpWanLink(st *State, device string) {
	link := WarpWanLink(device)
	for i, w := range st.Network.WanLinks {
		if w.ID == WanLinkIDWarp {
			st.Network.WanLinks[i] = link
			return
		}
	}
	st.Network.WanLinks = append(st.Network.WanLinks, link)
}

// RemoveWarpWanLink 移除 WARP WAN 链路行（保留引用 wan-warp 的出站策略，便于再次启用）。
func RemoveWarpWanLink(st *State) {
	var links []WanLink
	for _, w := range st.Network.WanLinks {
		if !IsWarpWanLink(w) {
			links = append(links, w)
		}
	}
	st.Network.WanLinks = links
}

// SetWarpEnabled 持久化 WARP 启用意图（与隧道瞬时状态解耦）。
func SetWarpEnabled(st *State, enabled bool) {
	st.Network.WarpEnabled = enabled
}

// SetWarpLicenseKey 持久化 WARP+ License Key（空字符串表示清除）。
func SetWarpLicenseKey(st *State, key string) {
	st.Network.WarpLicenseKey = strings.TrimSpace(key)
}

// WarpLicenseKeyConfigured 是否已保存 License Key。
func WarpLicenseKeyConfigured(st State) bool {
	return strings.TrimSpace(st.Network.WarpLicenseKey) != ""
}
