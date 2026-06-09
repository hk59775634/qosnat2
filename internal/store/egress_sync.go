package store

import (
	"fmt"
	"strings"
)

const egressRouteCommentPrefix = "qosnat-egress:"

// SyncEgressRoutes 将出站策略同步为各 WanLink 策略路由表中的 default 路由（写入 state.Routes）。
// 非主 WAN 链路始终写入其策略表；主 WAN 仅在存在出站策略引用时写入。
func SyncEgressRoutes(st *State) {
	var keep []RouteEntry
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, egressRouteCommentPrefix) ||
			strings.HasPrefix(r.Comment, wanPolicyRouteCommentPrefix) {
			continue
		}
		keep = append(keep, r)
	}

	refs := map[string]struct{}{}
	for _, p := range st.Network.EgressPolicies {
		if p.Enabled {
			refs[p.WanLinkID] = struct{}{}
		}
	}

	seen := map[string]struct{}{}
	for _, w := range st.Network.WanLinks {
		if !w.Enabled {
			continue
		}
		dev := strings.TrimSpace(w.Device)
		gw := strings.TrimSpace(w.Gateway)
		if dev == "" {
			continue
		}
		policyOnly := WanLinkUsesPolicyTableOnly(w, st.Network.WanLinks)
		_, referenced := refs[w.ID]
		if !policyOnly && !referenced {
			continue
		}
		tbl := WanLinkRouteTable(w.ID, st.Network.WanLinks)
		if tbl == 0 {
			continue
		}
		if _, ok := seen[w.ID]; ok {
			continue
		}
		seen[w.ID] = struct{}{}
		comment := wanPolicyRouteCommentPrefix + w.ID
		source := RouteSourceEgress
		note := fmt.Sprintf("策略 WAN · %s · 表%d", wanLinkDisplay(w), tbl)
		if referenced {
			comment = egressRouteCommentPrefix + w.ID
			note = fmt.Sprintf("出站 · %s · 表%d", wanLinkDisplay(w), tbl)
		}
		keep = append(keep, RouteEntry{
			ID:         "wan-policy-" + w.ID,
			Dest:       "default",
			Gateway:    gw,
			Device:     dev,
			Table:      tbl,
			Comment:    comment,
			Enabled:    true,
			Source:     source,
			Locked:     true,
			SourceNote: note,
		})
	}
	st.Routes = keep
}
