package route

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func routeDeviceReady(r store.RouteEntry) bool {
	if len(r.Nexthops) > 0 {
		for _, nh := range r.Nexthops {
			dev := strings.TrimSpace(nh.Device)
			if dev != "" && !netif.LinkExists(dev) {
				return false
			}
		}
		return true
	}
	dev := strings.TrimSpace(r.Device)
	if dev != "" && !netif.LinkExists(dev) {
		return false
	}
	return true
}

// PartitionByDeviceReady 将路由分为当前可下发与需等待出接口就绪的两组。
func PartitionByDeviceReady(routes []store.RouteEntry) (ready, deferred []store.RouteEntry) {
	for _, r := range routes {
		if !r.Enabled || routeDeviceReady(r) {
			ready = append(ready, r)
		} else {
			deferred = append(deferred, r)
		}
	}
	return ready, deferred
}

// DeferredRouteDevices 返回 deferred 路由中尚未存在的出接口名（去重）。
func DeferredRouteDevices(deferred []store.RouteEntry) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(dev string) {
		dev = strings.TrimSpace(dev)
		if dev == "" {
			return
		}
		if _, ok := seen[dev]; ok {
			return
		}
		seen[dev] = struct{}{}
		out = append(out, dev)
	}
	for _, r := range deferred {
		if len(r.Nexthops) > 0 {
			for _, nh := range r.Nexthops {
				add(nh.Device)
			}
			continue
		}
		add(r.Device)
	}
	return out
}
