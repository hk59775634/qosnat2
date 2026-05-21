package store

import "strings"

const wanRouteCommentPrefix = "qosnat-wan:"

// SyncWanRoutes 将启用的 WanLink 同步为 default 路由（保留非 WanLink 托管项）
func SyncWanRoutes(st *State) {
	var keep []RouteEntry
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, wanRouteCommentPrefix) {
			continue
		}
		keep = append(keep, r)
	}
	for _, w := range st.Network.WanLinks {
		if !w.Enabled || strings.TrimSpace(w.Gateway) == "" {
			continue
		}
		dest := "default"
		entry := RouteEntry{
			ID:      w.ID,
			Dest:    dest,
			Gateway: strings.TrimSpace(w.Gateway),
			Device:  strings.TrimSpace(w.Device),
			Metric:  w.Metric,
			Comment: wanRouteCommentPrefix + w.ID,
			Enabled: true,
		}
		if entry.Metric <= 0 {
			entry.Metric = 100 + w.Tier*10
		}
		keep = append(keep, entry)
	}
	st.Routes = keep
}
