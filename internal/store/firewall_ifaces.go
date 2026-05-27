package store

import (
	"fmt"
	"sort"
	"strings"
)

// FirewallIfaceInfo 防火墙 UI 可选网卡（类似 pfSense 接口 Tab）
type FirewallIfaceInfo struct {
	Name  string `json:"name"`
	Role  string `json:"role,omitempty"`  // LAN | WAN | VLAN | OPT
	Label string `json:"label,omitempty"` // 多 WAN 显示名等
}

// BuildFirewallIfaceList 汇总可用于防火墙规则筛选的网卡（去重、稳定排序）
func BuildFirewallIfaceList(st State, devLAN, devWAN string, systemDevices []string) []FirewallIfaceInfo {
	seen := map[string]struct{}{}
	var out []FirewallIfaceInfo

	add := func(name, role, label string) {
		name = strings.TrimSpace(name)
		if name == "" {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, FirewallIfaceInfo{Name: name, Role: role, Label: strings.TrimSpace(label)})
	}

	if devLAN != "" {
		add(devLAN, "LAN", "")
	}
	if devWAN != "" {
		add(devWAN, "WAN", "")
	}
	for _, w := range st.Network.WanLinks {
		if !w.Enabled {
			continue
		}
		role := "WAN"
		label := strings.TrimSpace(w.Name)
		if w.Device == devWAN {
			if label != "" {
				// 更新主 WAN 标签（若多 WAN 配置了名称）
				for i := range out {
					if out[i].Name == devWAN && out[i].Label == "" {
						out[i].Label = label
					}
				}
			}
			continue
		}
		add(w.Device, role, label)
	}
	for _, v := range st.Network.VLANs {
		nm := strings.TrimSpace(v.Name)
		if nm == "" && v.Parent != "" && v.VID > 0 {
			nm = fmt.Sprintf("%s.%d", v.Parent, v.VID)
		}
		add(nm, "VLAN", "")
	}
	for _, ic := range st.Network.Ifaces {
		add(ic.Device, "OPT", "")
	}
	for _, d := range systemDevices {
		d = strings.TrimSpace(d)
		if d == "" || d == "lo" {
			continue
		}
		if d == devLAN || d == devWAN {
			continue
		}
		already := false
		for _, x := range out {
			if x.Name == d {
				already = true
				break
			}
		}
		if !already {
			add(d, "OPT", "")
		}
	}

	sort.Slice(out, func(i, j int) bool {
		ri, rj := ifaceSortRank(out[i]), ifaceSortRank(out[j])
		if ri != rj {
			return ri < rj
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func ifaceSortRank(i FirewallIfaceInfo) int {
	switch i.Role {
	case "LAN":
		return 0
	case "WAN":
		return 1
	case "VLAN":
		return 2
	default:
		return 3
	}
}
