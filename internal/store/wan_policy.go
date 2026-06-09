package store

import (
	"sort"
	"strings"
)

const wanPolicyRouteCommentPrefix = "qosnat-wan-policy:"

// EnabledWanLinks 返回已启用且配置了 device 的 WanLink。
func EnabledWanLinks(links []WanLink) []WanLink {
	var out []WanLink
	for _, w := range links {
		if w.Enabled && strings.TrimSpace(w.Device) != "" {
			out = append(out, w)
		}
	}
	return out
}

type wanLinkRank struct {
	id     string
	metric int
	tier   int
}

// PrimaryWanLinkID 多 WAN 时主表 default 使用的链路（metric 最小，其次 tier，其次 id）。
func PrimaryWanLinkID(links []WanLink) string {
	enabled := EnabledWanLinks(links)
	if len(enabled) == 0 {
		return ""
	}
	ranks := make([]wanLinkRank, 0, len(enabled))
	for _, w := range enabled {
		m := w.Metric
		if m <= 0 {
			m = 100 + w.Tier*10
		}
		ranks = append(ranks, wanLinkRank{id: w.ID, metric: m, tier: w.Tier})
	}
	sort.Slice(ranks, func(i, j int) bool {
		if ranks[i].metric != ranks[j].metric {
			return ranks[i].metric < ranks[j].metric
		}
		if ranks[i].tier != ranks[j].tier {
			return ranks[i].tier < ranks[j].tier
		}
		return ranks[i].id < ranks[j].id
	})
	return ranks[0].id
}

// WanLinkUsesPolicyTableOnly 链路仅通过策略路由表出站（不参与 main default）。
func WanLinkUsesPolicyTableOnly(w WanLink, links []WanLink) bool {
	if w.PolicyOnly {
		return true
	}
	enabled := EnabledWanLinks(links)
	if len(enabled) <= 1 {
		return false
	}
	primary := PrimaryWanLinkID(links)
	return primary != "" && w.ID != primary
}
