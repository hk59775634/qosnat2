package route

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/hk59775634/qosnat2/internal/policyroute"
	"github.com/hk59775634/qosnat2/internal/store"
)

// ApplyResult 批量回放统计。
type ApplyResult struct {
	Applied int `json:"applied"`
	Skipped int `json:"skipped"`
	Backend string `json:"backend,omitempty"`
}

type liveIndex struct {
	byKey map[string]LiveRoute
}

func buildLiveIndex(routes []store.RouteEntry) (liveIndex, error) {
	tables := map[int]struct{}{254: {}}
	for _, r := range routes {
		if !r.Enabled {
			continue
		}
		tbl := r.Table
		if tbl == 0 {
			tbl = 254
		}
		tables[tbl] = struct{}{}
	}
	idx := liveIndex{byKey: map[string]LiveRoute{}}
	for tbl := range tables {
		live, err := ListLive(tbl)
		if err != nil {
			return liveIndex{}, err
		}
		for _, lr := range live {
			idx.byKey[store.RouteKey(lr.Dest, lr.Gateway, lr.Device, lr.Table)] = lr
		}
	}
	return idx, nil
}

func metricCompatible(want, live int) bool {
	// FRR/zebra 常把未指定 metric 的静态路由装成 metric 20；netplan 可能写成 0。
	// 任一侧为 0 时只按 dest/gw/dev/table 判断是否存在，避免误判 missing 导致反复 ApplyManaged 冲掉策略表。
	if want == 0 || live == 0 {
		return true
	}
	return want == live
}

func routeAlreadyApplied(r store.RouteEntry, idx liveIndex) bool {
	dest, err := store.NormalizeRouteDest(r.Dest)
	if err != nil {
		return false
	}
	table := r.Table
	if table == 0 {
		table = 254
	}
	if len(r.Nexthops) > 0 {
		// ECMP：内核常展开为多条同 dest/metric；metric 兼容且 nexthop 集合匹配则跳过。
		var matches int
		for _, nh := range r.Nexthops {
			gw := strings.TrimSpace(nh.Gateway)
			dev := strings.TrimSpace(nh.Device)
			k := store.RouteKey(dest, gw, dev, table)
			if lr, ok := idx.byKey[k]; ok && metricCompatible(r.Metric, lr.Metric) {
				matches++
			}
		}
		return matches == len(r.Nexthops)
	}
	gw := strings.TrimSpace(r.Gateway)
	dev := strings.TrimSpace(r.Device)
	k := store.RouteKey(dest, gw, dev, table)
	lr, ok := idx.byKey[k]
	if !ok {
		return false
	}
	return metricCompatible(r.Metric, lr.Metric)
}

func needsInfer(r store.RouteEntry) bool {
	if len(r.Nexthops) > 0 {
		for _, nh := range r.Nexthops {
			if strings.TrimSpace(nh.Device) == "" && strings.TrimSpace(nh.Gateway) != "" {
				return true
			}
		}
		return false
	}
	return strings.TrimSpace(r.Device) == "" && strings.TrimSpace(r.Gateway) != ""
}

// ApplyAllDiff 仅对变更项批量 ip route replace（单次 ip -batch），降低 netlink/FIB 抖动。
func ApplyAllDiff(routes []store.RouteEntry) (ApplyResult, error) {
	res := ApplyResult{Backend: "kernel"}
	idx, err := buildLiveIndex(routes)
	if err != nil {
		return res, err
	}
	var lines []string
	for _, r := range routes {
		if !r.Enabled {
			res.Skipped++
			continue
		}
		entry := r
		if needsInfer(entry) {
			entry = InferRouteDevices(entry)
		}
		if routeAlreadyApplied(entry, idx) {
			res.Skipped++
			continue
		}
		args, err := buildReplaceArgs(entry)
		if err != nil {
			return res, fmt.Errorf("%s: %w", entry.ID, err)
		}
		lines = append(lines, "route "+strings.Join(args[1:], " "))
	}
	if len(lines) == 0 {
		return res, nil
	}
	if err := runIPBatch(lines); err != nil {
		return res, err
	}
	res.Applied = len(lines)
	policyroute.FlushRouteCache()
	return res, nil
}

func runIPBatch(lines []string) error {
	sort.Strings(lines)
	f, err := os.CreateTemp("", "qosnat-ip-batch-*.conf")
	if err != nil {
		return err
	}
	path := f.Name()
	defer os.Remove(path)
	for _, line := range lines {
		if _, err := fmt.Fprintln(f, line); err != nil {
			f.Close()
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	out, err := exec.Command("ip", "-batch", path).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip -batch: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
