package store

import (
	"fmt"
	"strconv"
	"strings"
)

func wanLinkDisplay(w WanLink) string {
	if n := strings.TrimSpace(w.Name); n != "" {
		return n
	}
	return w.ID
}

func egressPolicyDisplay(p EgressPolicy) string {
	if n := strings.TrimSpace(p.Name); n != "" {
		return n
	}
	if lbl := EgressEndpointsLabel(p); lbl != "" {
		return lbl
	}
	return p.CIDR
}

func wanLinkRouteNote(w WanLink, metric int) string {
	if metric > 0 {
		return fmt.Sprintf("多 WAN · %s · m%d", wanLinkDisplay(w), metric)
	}
	return fmt.Sprintf("多 WAN · %s", wanLinkDisplay(w))
}

func wanRouteSourceNote(st State, r RouteEntry) string {
	suffix := strings.TrimPrefix(r.Comment, wanRouteCommentPrefix)
	if strings.HasPrefix(suffix, "nh-m") {
		metric := r.Metric
		if metric <= 0 {
			if i := strings.IndexByte(suffix, ':'); i > 0 {
				_, _ = fmt.Sscanf(suffix[:i], "nh-m%d", &metric)
			}
		}
		if metric > 0 {
			return fmt.Sprintf("多 WAN 负载 · m%d", metric)
		}
		return "多 WAN 负载"
	}
	if w, ok := FindWanLink(st.Network.WanLinks, suffix); ok {
		metric := r.Metric
		if metric <= 0 {
			metric = w.Metric
		}
		return wanLinkRouteNote(w, metric)
	}
	return "多 WAN"
}

func egressRouteSourceNote(st State, policyID string) string {
	for _, p := range st.Network.EgressPolicies {
		if p.ID != policyID {
			continue
		}
		w, ok := FindWanLink(st.Network.WanLinks, p.WanLinkID)
		tbl := WanLinkRouteTable(p.WanLinkID, st.Network.WanLinks)
		if !ok {
			return fmt.Sprintf("出站 · %s · 表%d", egressPolicyDisplay(p), tbl)
		}
		return fmt.Sprintf("出站 · %s → %s · 表%d", egressPolicyDisplay(p), wanLinkDisplay(w), tbl)
	}
	return "出站策略"
}

// EnrichRouteEntry 补全来源与说明（兼容旧数据未写入 source 字段）
func EnrichRouteEntry(r RouteEntry, st State) RouteEntry {
	if strings.HasPrefix(r.Comment, wanRouteCommentPrefix) {
		r.Source = RouteSourceWan
		r.Locked = true
		if r.SourceNote == "" {
			r.SourceNote = wanRouteSourceNote(st, r)
		}
		return r
	}
	if strings.HasPrefix(r.Comment, egressRouteCommentPrefix) {
		r.Source = RouteSourceEgress
		r.Locked = true
		if r.SourceNote == "" {
			id := strings.TrimPrefix(r.Comment, egressRouteCommentPrefix)
			r.SourceNote = egressRouteSourceNote(st, id)
		}
		return r
	}
	if r.Source == "" {
		r.Source = RouteSourceManual
	}
	return r
}

// EnrichManagedRoutes 返回带 source/source_note/locked 的托管路由列表
func EnrichManagedRoutes(routes []RouteEntry, st State) []RouteEntry {
	if routes == nil {
		return []RouteEntry{}
	}
	out := make([]RouteEntry, len(routes))
	for i, r := range routes {
		out[i] = EnrichRouteEntry(r, st)
	}
	return out
}

// RouteTableLabel 路由表显示名（main 或策略表编号）
func RouteTableLabel(table int) string {
	if table <= 0 || table == 254 {
		return "main"
	}
	return "表 " + strconv.Itoa(table)
}
