package store

import (
	"fmt"
	"strings"
)

const egressRouteCommentPrefix = "qosnat-egress:"

// SyncEgressRoutes 将出站策略同步为各 WanLink 路由表中的 default 路由（写入 state.Routes）
func SyncEgressRoutes(st *State) {
	var keep []RouteEntry
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, egressRouteCommentPrefix) {
			continue
		}
		keep = append(keep, r)
	}
	for _, p := range st.Network.EgressPolicies {
		if !p.Enabled {
			continue
		}
		w, ok := FindWanLink(st.Network.WanLinks, p.WanLinkID)
		if !ok || !w.Enabled {
			continue
		}
		dev := strings.TrimSpace(w.Device)
		gw := strings.TrimSpace(w.Gateway)
		if dev == "" || gw == "" {
			continue
		}
		tbl := WanLinkRouteTable(w.ID, st.Network.WanLinks)
		if tbl == 0 {
			continue
		}
		keep = append(keep, RouteEntry{
			ID:         p.ID,
			Dest:       "default",
			Gateway:    gw,
			Device:     dev,
			Table:      tbl,
			Comment:    egressRouteCommentPrefix + p.ID,
			Enabled:    true,
			Source:     RouteSourceEgress,
			Locked:     true,
			SourceNote: fmt.Sprintf("出站 · %s → %s · 表%d", egressPolicyDisplay(p), wanLinkDisplay(w), tbl),
		})
	}
	st.Routes = keep
}
