package store

import (
	"fmt"
	"sort"
	"strings"
)

// Auto rule IDs (reserved prefix auto-).
const (
	autoIDInputAdmin     = "auto-input-admin"
	autoIDInputOcservTCP = "auto-input-ocserv-tcp"
	autoIDInputOcservUDP = "auto-input-ocserv-udp"
	autoIDInputWanDrop   = "auto-input-wan-drop"
)

// AutoInputVPN mirrors nft.VPNFirewall without importing nft.
type AutoInputVPN struct {
	OCServEnabled bool
	OCServTCP     int
	OCServUDP     int
	WGPorts       []int
}

// IsAutoManagedRule 平台自动同步的受管规则（不可编辑/删除/排序）。
func IsAutoManagedRule(r FilterRule) bool {
	if r.System {
		return true
	}
	id := strings.TrimSpace(r.ID)
	return strings.HasPrefix(id, "auto-")
}

// CollectWanInputDevices 返回需生成 WAN 入站自动规则的网卡（主 WAN + 已启用 WanLink，排除 LAN）。
func CollectWanInputDevices(devWAN, devLAN string, st State) []string {
	devLAN = strings.TrimSpace(devLAN)
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == devLAN {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	add(devWAN)
	for _, w := range st.Network.WanLinks {
		if !w.Enabled {
			continue
		}
		add(w.Device)
	}
	sort.Strings(out)
	return out
}

// CollectWanForwardDevices 返回 forward 链需处理的 WAN 口（主 WAN + 已启用 WanLink + 已解析 egress，排除 LAN）。
func CollectWanForwardDevices(devWAN, devLAN string, st State, egressDevices []string) []string {
	devLAN = strings.TrimSpace(devLAN)
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == devLAN {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, d := range CollectWanInputDevices(devWAN, devLAN, st) {
		add(d)
	}
	for _, d := range egressDevices {
		add(d)
	}
	sort.Strings(out)
	return out
}

// BuildAutoInputRules 为每个 WAN 口生成 input 链自动规则（放行项 → 该口默认丢弃）。
func BuildAutoInputRules(wanDevs []string, adminPort string, vpn AutoInputVPN) []FilterRule {
	if len(wanDevs) == 0 {
		return nil
	}
	port := strings.TrimSpace(adminPort)
	if port == "" {
		port = "8080"
	}
	adminP := parsePortInt(port)
	var out []FilterRule
	for _, wan := range wanDevs {
		wan = strings.TrimSpace(wan)
		if wan == "" {
			continue
		}
		sfx := wan
		out = append(out, FilterRule{
			ID:      autoIDInputAdmin + "-" + sfx,
			Chain:   "input",
			Action:  "accept",
			Iif:     wan,
			Proto:   "tcp",
			DstPort: adminP,
			Comment: fmt.Sprintf("qosnat2 管理端口 %s（自动）", wan),
			Enabled: true,
			System:  true,
		})
		if vpn.OCServEnabled && vpn.OCServTCP > 0 {
			out = append(out, FilterRule{
				ID: autoIDInputOcservTCP + "-" + sfx, Chain: "input", Action: "accept",
				Iif: wan, Proto: "tcp", DstPort: vpn.OCServTCP,
				Comment: fmt.Sprintf("OpenConnect TCP %s（自动）", wan), Enabled: true, System: true,
			})
		}
		if vpn.OCServEnabled && vpn.OCServUDP > 0 {
			out = append(out, FilterRule{
				ID: autoIDInputOcservUDP + "-" + sfx, Chain: "input", Action: "accept",
				Iif: wan, Proto: "udp", DstPort: vpn.OCServUDP,
				Comment: fmt.Sprintf("OpenConnect UDP %s（自动）", wan), Enabled: true, System: true,
			})
		}
		for _, p := range vpn.WGPorts {
			if p <= 0 {
				continue
			}
			out = append(out, FilterRule{
				ID:      fmt.Sprintf("auto-input-wg-%d-%s", p, sfx),
				Chain:   "input",
				Action:  "accept",
				Iif:     wan,
				Proto:   "udp",
				DstPort: p,
				Comment: fmt.Sprintf("WireGuard UDP/%d %s（自动）", p, wan),
				Enabled: true,
				System:  true,
			})
		}
		out = append(out, FilterRule{
			ID: autoIDInputWanDrop + "-" + sfx, Chain: "input", Action: "drop",
			Iif: wan, Comment: fmt.Sprintf("WAN 入站默认丢弃 %s（自动）", wan), Enabled: true, System: true,
		})
	}
	return out
}

func parsePortInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n <= 0 || n > 65535 {
		return 8080
	}
	return n
}

func filterRulesEqual(a, b []FilterRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !autoRuleEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func autoRuleEqual(a, b FilterRule) bool {
	return a.ID == b.ID && a.Chain == b.Chain && a.Action == b.Action &&
		a.Iif == b.Iif && a.Oif == b.Oif && a.Proto == b.Proto &&
		a.SrcAddr == b.SrcAddr && a.DstAddr == b.DstAddr &&
		a.SrcAlias == b.SrcAlias && a.DstAlias == b.DstAlias &&
		a.SrcPort == b.SrcPort && a.DstPort == b.DstPort &&
		a.IPVersion == b.IPVersion &&
		a.Comment == b.Comment && a.Enabled == b.Enabled && a.System == b.System
}

// splitAutoInputByDrop 将 input 自动规则拆为放行段与 WAN 默认丢弃段。
func splitAutoInputByDrop(rules []FilterRule) (accepts, drops []FilterRule) {
	for _, r := range rules {
		if strings.HasPrefix(strings.TrimSpace(r.ID), autoIDInputWanDrop) {
			drops = append(drops, r)
		} else {
			accepts = append(accepts, r)
		}
	}
	return accepts, drops
}

// SyncAutoFilterRules 合并用户规则与自动规则。
// forward：用户 → 端口转发自动规则。
// input：自动放行(admin/VPN) → 用户 → 自动 WAN 丢弃（管理口/VPN 不被用户 drop 覆盖）。
func SyncAutoFilterRules(rules []FilterRule, wanDevs []string, adminPort string, vpn AutoInputVPN, forwards []WanPortForward, devLAN string) ([]FilterRule, bool) {
	desiredFwd := BuildAutoForwardFilterRules(forwards, devLAN)
	autoInputAccept, autoInputDrop := splitAutoInputByDrop(BuildAutoInputRules(wanDevs, adminPort, vpn))
	var userFwd, userInput []FilterRule
	for _, r := range rules {
		if IsAutoManagedRule(r) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(r.Chain)) == "input" {
			userInput = append(userInput, r)
		} else {
			userFwd = append(userFwd, r)
		}
	}
	merged := append(append(append(append(
		append([]FilterRule{}, userFwd...),
		desiredFwd...),
		autoInputAccept...),
		userInput...),
		autoInputDrop...)
	return merged, !filterRulesEqual(merged, rules)
}
