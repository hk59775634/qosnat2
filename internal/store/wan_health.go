package store

import "strings"

// WanLinkMonitorAddr 返回探测目标地址。
func WanLinkMonitorAddr(w WanLink) string {
	if a := strings.TrimSpace(w.MonitorAddr); a != "" {
		return a
	}
	return strings.TrimSpace(w.Gateway)
}

// WanLinkMonitorInterval 返回探测间隔秒数（下限 2）。
func WanLinkMonitorInterval(w WanLink) int {
	n := w.MonitorIntervalSec
	if n <= 0 {
		return 5
	}
	if n < 2 {
		return 2
	}
	return n
}

// WanLinkMonitorLossThreshold 返回连续失败阈值（下限 1）。
func WanLinkMonitorLossThreshold(w WanLink) int {
	n := w.MonitorLossThreshold
	if n <= 0 {
		return 3
	}
	return n
}

// FilterWanLinksForRouting 排除健康探测判定为 down 的链路（unhealthy 为 WanLink.ID 集合）。
func FilterWanLinksForRouting(links []WanLink, unhealthy map[string]bool) []WanLink {
	if len(unhealthy) == 0 {
		return links
	}
	out := make([]WanLink, 0, len(links))
	for _, w := range links {
		if unhealthy[w.ID] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// ApplyWanHealthToState 将不健康链路从路由同步视角临时剔除（不改 Enabled）。
func ApplyWanHealthToState(st *State, unhealthy map[string]bool) {
	if st == nil || len(unhealthy) == 0 {
		return
	}
	st.Network.WanLinks = FilterWanLinksForRouting(st.Network.WanLinks, unhealthy)
}
