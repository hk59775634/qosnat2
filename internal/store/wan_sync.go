package store

import (
	"sort"
	"strconv"
	"strings"
)

const wanRouteCommentPrefix = "qosnat-wan:"

// SyncWanRoutes 将启用的 WanLink 同步为 default 路由（保留非 WanLink 托管项）。
// 同一 metric 的多条链路合并为一条 nexthop 路由，Weight 映射为 ip route weight。
func SyncWanRoutes(st *State) {
	var keep []RouteEntry
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, wanRouteCommentPrefix) {
			continue
		}
		keep = append(keep, r)
	}
	type linkMetric struct {
		w      WanLink
		metric int
	}
	var enabled []linkMetric
	for _, w := range st.Network.WanLinks {
		if !w.Enabled || strings.TrimSpace(w.Gateway) == "" {
			continue
		}
		m := w.Metric
		if m <= 0 {
			m = 100 + w.Tier*10
		}
		enabled = append(enabled, linkMetric{w: w, metric: m})
	}
	sort.Slice(enabled, func(i, j int) bool {
		if enabled[i].metric != enabled[j].metric {
			return enabled[i].metric < enabled[j].metric
		}
		return enabled[i].w.ID < enabled[j].w.ID
	})
	byMetric := map[int][]WanLink{}
	order := []int{}
	for _, lm := range enabled {
		if _, ok := byMetric[lm.metric]; !ok {
			order = append(order, lm.metric)
		}
		byMetric[lm.metric] = append(byMetric[lm.metric], lm.w)
	}
	sort.Ints(order)
	for _, metric := range order {
		links := byMetric[metric]
		if len(links) == 1 {
			w := links[0]
			keep = append(keep, RouteEntry{
				ID:      w.ID,
				Dest:    "default",
				Gateway: strings.TrimSpace(w.Gateway),
				Device:  strings.TrimSpace(w.Device),
				Metric:  metric,
				Comment: wanRouteCommentPrefix + w.ID,
				Enabled: true,
			})
			continue
		}
		var nh []RouteNexthop
		var ids []string
		for _, w := range links {
			weight := w.Weight
			if weight <= 0 {
				weight = 1
			}
			nh = append(nh, RouteNexthop{
				Gateway: strings.TrimSpace(w.Gateway),
				Device:  strings.TrimSpace(w.Device),
				Weight:  weight,
			})
			ids = append(ids, w.ID)
		}
		sort.Strings(ids)
		keep = append(keep, RouteEntry{
			ID:       "wan-nh-" + strconv.Itoa(metric),
			Dest:     "default",
			Nexthops: nh,
			Metric:   metric,
			Comment:  wanRouteCommentPrefix + "nh-m" + strconv.Itoa(metric) + ":" + strings.Join(ids, "+"),
			Enabled:  true,
		})
	}
	st.Routes = keep
}
