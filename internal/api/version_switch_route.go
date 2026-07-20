package api

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/policyroute"
	"github.com/hk59775634/qosnat2/internal/releasecatalog"
	"github.com/hk59775634/qosnat2/internal/store"
)

const versionSwitchTempRulePriority = 45

type versionSwitchTempRules struct {
	cidrs       []string
	table       int
	priority    int
	tempDefault bool
	wanDevice   string
}

func buildDownloadRouteOptions(st store.State) []map[string]any {
	opts := []map[string]any{
		{"id": releasecatalog.RouteDirect},
		{"id": releasecatalog.RouteGHProxyV4},
		{"id": releasecatalog.RouteGHProxyCDN},
	}
	wans := eligibleVersionSwitchWanLinks(st.Network.WanLinks)
	for i, w := range wans {
		if i >= 2 {
			break
		}
		routeID := releasecatalog.RouteWan1
		if i == 1 {
			routeID = releasecatalog.RouteWan2
		}
		opts = append(opts, map[string]any{
			"id":          routeID,
			"wan_index":   i + 1,
			"wan_link_id": w.ID,
			"wan_name":    strings.TrimSpace(w.Name),
			"device":      w.Device,
			"gateway":     strings.TrimSpace(w.Gateway),
		})
	}
	return opts
}

func eligibleVersionSwitchWanLinks(links []store.WanLink) []store.WanLink {
	var out []store.WanLink
	for _, w := range links {
		if !w.Enabled || store.IsManagedWanLink(w) {
			continue
		}
		if strings.TrimSpace(w.Device) == "" {
			continue
		}
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Tier != out[j].Tier {
			return out[i].Tier < out[j].Tier
		}
		if out[i].Metric != out[j].Metric {
			return out[i].Metric < out[j].Metric
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func wanLinkForDownloadRoute(st store.State, route string) (store.WanLink, bool) {
	idx := releasecatalog.WanRouteIndex(route)
	if idx <= 0 {
		return store.WanLink{}, false
	}
	wans := eligibleVersionSwitchWanLinks(st.Network.WanLinks)
	if len(wans) < idx {
		return store.WanLink{}, false
	}
	return wans[idx-1], true
}

func resolveDownloadHostCIDRs(hosts []string) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	resolver := net.Resolver{PreferGo: true}
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
		ips, err := resolver.LookupIP(ctx, "ip4", host)
		cancel()
		if err != nil {
			return nil, fmt.Errorf("resolve %s: %w", host, err)
		}
		for _, ip := range ips {
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			cidr := v4.String() + "/32"
			if _, ok := seen[cidr]; ok {
				continue
			}
			seen[cidr] = struct{}{}
			out = append(out, cidr)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no IPv4 addresses resolved for download hosts")
	}
	sort.Strings(out)
	return out, nil
}

func applyVersionSwitchTempEgress(st store.State, route string) (*versionSwitchTempRules, error) {
	if !releasecatalog.UsesWanEgress(route) {
		return nil, nil
	}
	w, ok := wanLinkForDownloadRoute(st, route)
	if !ok {
		return nil, fmt.Errorf("download route %s: no eligible WAN link (configure at least one enabled non-WARP WAN)", route)
	}
	tbl := store.WanLinkRouteTable(w.ID, st.Network.WanLinks)
	if tbl <= 0 {
		return nil, fmt.Errorf("download route %s: wan link %q has no route table", route, w.ID)
	}
	hosts := releasecatalog.DownloadHostnames(route)
	cidrs, err := resolveDownloadHostCIDRs(hosts)
	if err != nil {
		return nil, err
	}
	tr := &versionSwitchTempRules{cidrs: cidrs, table: tbl, priority: versionSwitchTempRulePriority, wanDevice: w.Device}
	if err := ensureWanTableDefaultRoute(w, tbl); err != nil {
		return nil, err
	}
	tr.tempDefault = true
	for _, cidr := range cidrs {
		if err := policyroute.AddDestinationRules(cidr, tbl, tr.priority); err != nil {
			removeVersionSwitchTempEgress(tr)
			return nil, fmt.Errorf("temp egress %s via %s: %w", cidr, w.Device, err)
		}
	}
	policyroute.FlushRouteCache()
	return tr, nil
}

func removeVersionSwitchTempEgress(tr *versionSwitchTempRules) {
	if tr == nil {
		return
	}
	for _, cidr := range tr.cidrs {
		policyroute.DeleteDestinationRules(cidr, tr.table, tr.priority)
	}
	if tr.tempDefault && tr.table > 0 {
		_ = exec.Command("ip", "route", "del", "default", "table", strconv.Itoa(tr.table)).Run()
	}
	policyroute.FlushRouteCache()
}

func ensureWanTableDefaultRoute(w store.WanLink, table int) error {
	dev := strings.TrimSpace(w.Device)
	if dev == "" || table <= 0 {
		return fmt.Errorf("wan link missing device or route table")
	}
	args := []string{"route", "replace", "default"}
	if gw := strings.TrimSpace(w.Gateway); gw != "" {
		args = append(args, "via", gw)
	}
	args = append(args, "dev", dev, "table", strconv.Itoa(table))
	if out, err := exec.Command("ip", args...).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("ip %s: %s", strings.Join(args, " "), msg)
	}
	return nil
}
