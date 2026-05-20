package route

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

// LiveRoute 内核路由表项（ip -json route）
type LiveRoute struct {
	Dest     string   `json:"dest"`
	Gateway  string   `json:"gateway,omitempty"`
	Device   string   `json:"device,omitempty"`
	Protocol string   `json:"protocol,omitempty"`
	Scope    string   `json:"scope,omitempty"`
	Metric   int      `json:"metric,omitempty"`
	PrefSrc  string   `json:"prefsrc,omitempty"`
	Flags    []string `json:"flags,omitempty"`
	Table    int      `json:"table"`
	Managed  bool     `json:"managed"`
}

type ipRouteJSON struct {
	Dst      string   `json:"dst"`
	Gateway  string   `json:"gateway"`
	Dev      string   `json:"dev"`
	Protocol string   `json:"protocol"`
	Scope    string   `json:"scope"`
	PrefSrc  string   `json:"prefsrc"`
	Metric   int      `json:"metric"`
	Flags    []string `json:"flags"`
}

// ListLive 读取指定路由表
func ListLive(table int) ([]LiveRoute, error) {
	if table == 0 {
		table = 254
	}
	args := []string{"-json", "route", "show", "table", strconv.Itoa(table)}
	out, err := exec.Command("ip", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("ip route show: %w", err)
	}
	var raw []ipRouteJSON
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var list []LiveRoute
	for _, r := range raw {
		dest := r.Dst
		if dest == "" {
			dest = "default"
		}
		list = append(list, LiveRoute{
			Dest: dest, Gateway: r.Gateway, Device: r.Dev,
			Protocol: r.Protocol, Scope: r.Scope, Metric: r.Metric,
			PrefSrc: r.PrefSrc, Flags: r.Flags, Table: table,
		})
	}
	return list, nil
}

// MarkManaged 标记哪些 live 路由由 state 托管
func MarkManaged(live []LiveRoute, managed []store.RouteEntry) []LiveRoute {
	keys := map[string]struct{}{}
	for _, m := range managed {
		if !m.Enabled {
			continue
		}
		dest, _ := store.NormalizeRouteDest(m.Dest)
		keys[store.RouteKey(dest, m.Gateway, m.Device, m.Table)] = struct{}{}
	}
	for i := range live {
		k := store.RouteKey(live[i].Dest, live[i].Gateway, live[i].Device, live[i].Table)
		if _, ok := keys[k]; ok {
			live[i].Managed = true
		}
	}
	return live
}

// Apply 应用单条托管路由
func Apply(r store.RouteEntry) error {
	if !r.Enabled {
		return nil
	}
	args, err := buildReplaceArgs(r)
	if err != nil {
		return err
	}
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip %s: %s %w", strings.Join(args, " "), strings.TrimSpace(string(out)), err)
	}
	return nil
}

// Delete 删除内核中的路由
func Delete(r store.RouteEntry) error {
	args, err := buildDelArgs(r)
	if err != nil {
		return err
	}
	out, err := exec.Command("ip", args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" && !strings.Contains(msg, "No such process") && !strings.Contains(strings.ToLower(msg), "not found") {
			return fmt.Errorf("ip %s: %s %w", strings.Join(args, " "), msg, err)
		}
	}
	return nil
}

// ApplyAll 回放 state 中启用的路由
func ApplyAll(routes []store.RouteEntry) error {
	for _, r := range routes {
		if !r.Enabled {
			continue
		}
		if err := Apply(r); err != nil {
			return fmt.Errorf("%s: %w", r.ID, err)
		}
	}
	return nil
}

func buildReplaceArgs(r store.RouteEntry) ([]string, error) {
	dest, err := store.NormalizeRouteDest(r.Dest)
	if err != nil {
		return nil, err
	}
	args := []string{"route", "replace", dest}
	if g := strings.TrimSpace(r.Gateway); g != "" {
		args = append(args, "via", g)
	}
	if d := strings.TrimSpace(r.Device); d != "" {
		args = append(args, "dev", d)
	}
	if r.Metric > 0 {
		args = append(args, "metric", strconv.Itoa(r.Metric))
	}
	table := r.Table
	if table == 0 {
		table = 254
	}
	if table != 254 {
		args = append(args, "table", strconv.Itoa(table))
	}
	return args, nil
}

func buildDelArgs(r store.RouteEntry) ([]string, error) {
	dest, err := store.NormalizeRouteDest(r.Dest)
	if err != nil {
		return nil, err
	}
	args := []string{"route", "del", dest}
	if g := strings.TrimSpace(r.Gateway); g != "" {
		args = append(args, "via", g)
	}
	if d := strings.TrimSpace(r.Device); d != "" {
		args = append(args, "dev", d)
	}
	table := r.Table
	if table == 0 {
		table = 254
	}
	if table != 254 {
		args = append(args, "table", strconv.Itoa(table))
	}
	return args, nil
}

// LinkExists 检查接口是否存在
func LinkExists(name string) bool {
	return netif.LinkExists(name)
}
