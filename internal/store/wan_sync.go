package store

import (
	"fmt"
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
	ifaceMainDevs := map[string]struct{}{}
	for _, r := range keep {
		if !r.Enabled || !strings.HasPrefix(r.Comment, ifaceGwRouteCommentPrefix) {
			continue
		}
		dest, _ := NormalizeRouteDest(r.Dest)
		if dest != "default" {
			continue
		}
		tbl := r.Table
		if tbl == 0 {
			tbl = 254
		}
		if tbl != 254 {
			continue
		}
		if dev := strings.TrimSpace(r.Device); dev != "" {
			ifaceMainDevs[dev] = struct{}{}
		}
	}
	for _, w := range st.Network.WanLinks {
		if !w.Enabled || WanLinkUsesPolicyTableOnly(w, st.Network.WanLinks) || strings.TrimSpace(w.Device) == "" {
			continue
		}
		// 主表 default 已由接口网关（iface-gw）提供时，其它设备上的 WanLink 只走策略表，
		// 避免再写入无法装入 FIB 的 main default（如 wg0），触发 FRR 反复 no+add 冲掉出站策略路由。
		if len(ifaceMainDevs) > 0 {
			if _, ok := ifaceMainDevs[strings.TrimSpace(w.Device)]; !ok {
				continue
			}
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
				ID:         w.ID,
				Dest:       "default",
				Gateway:    strings.TrimSpace(w.Gateway),
				Device:     strings.TrimSpace(w.Device),
				Metric:     metric,
				Comment:    wanRouteCommentPrefix + w.ID,
				Enabled:    true,
				Source:     RouteSourceWan,
				Locked:     true,
				SourceNote: wanLinkRouteNote(w, metric),
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
			ID:         "wan-nh-" + strconv.Itoa(metric),
			Dest:       "default",
			Nexthops:   nh,
			Metric:     metric,
			Comment:    wanRouteCommentPrefix + "nh-m" + strconv.Itoa(metric) + ":" + strings.Join(ids, "+"),
			Enabled:    true,
			Source:     RouteSourceWan,
			Locked:     true,
			SourceNote: fmt.Sprintf("多 WAN 负载 · m%d", metric),
		})
	}
	st.Routes = keep
}
