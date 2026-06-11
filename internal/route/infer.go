package route

import (
	"encoding/json"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

type routeGetJSON struct {
	Dev string `json:"dev"`
}

// InferDeviceForGateway 用 ip route get 解析网关应走的出接口（on-link 时内核需 dev）。
func InferDeviceForGateway(gateway string) string {
	gateway = strings.TrimSpace(gateway)
	if gateway == "" {
		return ""
	}
	out, err := exec.Command("ip", "-json", "route", "get", gateway).Output()
	if err != nil {
		return ""
	}
	var rows []routeGetJSON
	if err := json.Unmarshal(out, &rows); err != nil || len(rows) == 0 {
		return ""
	}
	dev := strings.TrimSpace(rows[0].Dev)
	// 本地/环回解析不应作为静态路由出接口（仅填网关时内核与 FRR 均可省略 dev）。
	if dev == "" || dev == "lo" {
		return ""
	}
	return dev
}

// InferRouteDevices 为缺少 device 的网关/nexthop 补全出接口（不修改原 slice 外的共享状态）。
func InferRouteDevices(r store.RouteEntry) store.RouteEntry {
	if len(r.Nexthops) > 0 {
		out := r
		out.Nexthops = append([]store.RouteNexthop(nil), r.Nexthops...)
		for i := range out.Nexthops {
			nh := &out.Nexthops[i]
			if strings.TrimSpace(nh.Device) != "" || strings.TrimSpace(nh.Gateway) == "" {
				continue
			}
			if dev := InferDeviceForGateway(nh.Gateway); dev != "" {
				nh.Device = dev
			}
		}
		return out
	}
	if strings.TrimSpace(r.Device) == "" && strings.TrimSpace(r.Gateway) != "" {
		if dev := InferDeviceForGateway(r.Gateway); dev != "" {
			r.Device = dev
		}
	}
	return r
}
