package store

import (
	"fmt"
	"strings"
)

// Auto rule IDs (reserved prefix auto-).
const (
	autoIDInputAdmin    = "auto-input-admin"
	autoIDInputOcservTCP = "auto-input-ocserv-tcp"
	autoIDInputOcservUDP = "auto-input-ocserv-udp"
	autoIDInputWanDrop  = "auto-input-wan-drop"
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

// BuildAutoInputRules 根据当前 WAN/管理端口/VPN 生成 input 链自动规则（顺序：放行项 → WAN 默认丢弃）。
func BuildAutoInputRules(devWan, adminPort string, vpn AutoInputVPN) []FilterRule {
	wan := strings.TrimSpace(devWan)
	if wan == "" {
		return nil
	}
	port := strings.TrimSpace(adminPort)
	if port == "" {
		port = "8080"
	}
	var out []FilterRule
	out = append(out, FilterRule{
		ID:      autoIDInputAdmin,
		Chain:   "input",
		Action:  "accept",
		Iif:     wan,
		Proto:   "tcp",
		DstPort: parsePortInt(port),
		Comment: "qosnat2 管理端口（自动）",
		Enabled: true,
		System:  true,
	})
	if vpn.OCServEnabled && vpn.OCServTCP > 0 {
		out = append(out, FilterRule{
			ID: autoIDInputOcservTCP, Chain: "input", Action: "accept",
			Iif: wan, Proto: "tcp", DstPort: vpn.OCServTCP,
			Comment: "OpenConnect TCP（自动）", Enabled: true, System: true,
		})
	}
	if vpn.OCServEnabled && vpn.OCServUDP > 0 {
		out = append(out, FilterRule{
			ID: autoIDInputOcservUDP, Chain: "input", Action: "accept",
			Iif: wan, Proto: "udp", DstPort: vpn.OCServUDP,
			Comment: "OpenConnect UDP（自动）", Enabled: true, System: true,
		})
	}
	for _, p := range vpn.WGPorts {
		if p <= 0 {
			continue
		}
		out = append(out, FilterRule{
			ID:      fmt.Sprintf("auto-input-wg-%d", p),
			Chain:   "input",
			Action:  "accept",
			Iif:     wan,
			Proto:   "udp",
			DstPort: p,
			Comment: fmt.Sprintf("WireGuard UDP/%d（自动）", p),
			Enabled: true,
			System:  true,
		})
	}
	out = append(out, FilterRule{
		ID: autoIDInputWanDrop, Chain: "input", Action: "drop",
		Iif: wan, Comment: "WAN 入站默认丢弃（自动）", Enabled: true, System: true,
	})
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
		a.Comment == b.Comment && a.Enabled == b.Enabled && a.System == b.System
}

// SyncAutoFilterRules 合并用户规则与自动规则；自动规则固定在数组末尾（input 链在 forward 之后）。
func SyncAutoFilterRules(rules []FilterRule, devWan, adminPort string, vpn AutoInputVPN) ([]FilterRule, bool) {
	desired := BuildAutoInputRules(devWan, adminPort, vpn)
	var user []FilterRule
	for _, r := range rules {
		if !IsAutoManagedRule(r) {
			user = append(user, r)
		}
	}
	merged := append(append([]FilterRule{}, user...), desired...)
	return merged, !filterRulesEqual(merged, rules)
}
