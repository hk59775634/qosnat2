package store

import (
	"fmt"
	"net"
	"sort"
	"strings"
)

// NormalizeIPv4PolicyCIDR 将 IPv4 主机或 CIDR 规范为策略路由项。
func NormalizeIPv4PolicyCIDR(s string) (string, error) {
	return normalizeEgressIPv4CIDR(s)
}

// CIDRCoveredByExisting 判断 cidr 是否已被 routes 中某项完整覆盖（含精确相等）。
func CIDRCoveredByExisting(routes []string, cidr string) bool {
	_, inner, err := net.ParseCIDR(strings.TrimSpace(cidr))
	if err != nil {
		return false
	}
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if r == cidr {
			return true
		}
		_, outer, err := net.ParseCIDR(r)
		if err != nil {
			continue
		}
		if cidrContainedIn(inner, outer) {
			return true
		}
	}
	return false
}

func cidrContainedIn(inner, outer *net.IPNet) bool {
	if inner == nil || outer == nil {
		return false
	}
	onesInner, _ := inner.Mask.Size()
	onesOuter, _ := outer.Mask.Size()
	return onesOuter <= onesInner && outer.Contains(inner.IP)
}

// PruneContainedPolicyRoutes 去掉被同列表中更宽网段完全覆盖的冗余项。
func PruneContainedPolicyRoutes(routes []string) []string {
	type item struct {
		cidr  string
		ones  int
		ipNet *net.IPNet
	}
	var items []item
	seen := map[string]struct{}{}
	for _, r := range routes {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		if _, ok := seen[r]; ok {
			continue
		}
		_, n, err := net.ParseCIDR(r)
		if err != nil {
			continue
		}
		ones, _ := n.Mask.Size()
		items = append(items, item{cidr: r, ones: ones, ipNet: n})
		seen[r] = struct{}{}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].ones != items[j].ones {
			return items[i].ones < items[j].ones
		}
		return items[i].cidr < items[j].cidr
	})
	var out []string
	for i, cur := range items {
		redundant := false
		for j := 0; j < i; j++ {
			if cidrContainedIn(cur.ipNet, items[j].ipNet) {
				redundant = true
				break
			}
		}
		if !redundant {
			out = append(out, cur.cidr)
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func mappingPolicyCIDRs(n NatIPv4State) ([]string, error) {
	seen := map[string]struct{}{}
	var out []string
	add := func(c string) error {
		c = strings.TrimSpace(c)
		if c == "" {
			return nil
		}
		if _, ok := seen[c]; ok {
			return nil
		}
		seen[c] = struct{}{}
		out = append(out, c)
		return nil
	}
	for inner := range n.StaticMappings {
		c, err := NormalizeIPv4PolicyCIDR(inner)
		if err != nil {
			return nil, fmt.Errorf("static mapping %q: %w", inner, err)
		}
		if err := add(c); err != nil {
			return nil, err
		}
	}
	for inner := range n.PrefixMappings {
		c, err := NormalizeIPv4PolicyCIDR(inner)
		if err != nil {
			return nil, fmt.Errorf("prefix mapping %q: %w", inner, err)
		}
		if err := add(c); err != nil {
			return nil, err
		}
	}
	return out, nil
}

func policyRouteManual(n NatIPv4State) []string {
	return subtractPolicyRoutes(subtractPolicyRoutes(n.PolicyRoutes, n.AutoPolicyRoutes), n.AutoVPNPolicyRoutes)
}

func rebuildPolicyRoutes(n *NatIPv4State, manual []string) {
	n.PolicyRoutes = PruneContainedPolicyRoutes(append(append(append([]string(nil), manual...), n.AutoPolicyRoutes...), n.AutoVPNPolicyRoutes...))
}

// RefreshMappingPolicyRoutes 根据 1:1 / 网段映射同步 auto_policy_routes，并清理冗余策略网段。
func RefreshMappingPolicyRoutes(n *NatIPv4State) error {
	if n == nil {
		return nil
	}
	if n.AutoPolicyRoutes == nil {
		n.AutoPolicyRoutes = []string{}
	}
	if n.AutoVPNPolicyRoutes == nil {
		n.AutoVPNPolicyRoutes = []string{}
	}
	needed, err := mappingPolicyCIDRs(*n)
	if err != nil {
		return err
	}
	manual := policyRouteManual(*n)
	var nextAuto []string
	for _, cidr := range needed {
		if CIDRCoveredByExisting(manual, cidr) || CIDRCoveredByExisting(n.AutoVPNPolicyRoutes, cidr) || CIDRCoveredByExisting(nextAuto, cidr) {
			continue
		}
		nextAuto = append(nextAuto, cidr)
	}
	n.AutoPolicyRoutes = nextAuto
	rebuildPolicyRoutes(n, manual)
	return nil
}

func appendIPv4PolicyCIDR(seen map[string]struct{}, out *[]string, raw string) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return
	}
	var cidr string
	if ip, n, err := net.ParseCIDR(raw); err == nil && ip != nil && n != nil && n.IP.To4() != nil {
		cidr = n.String()
	} else if ip := net.ParseIP(raw); ip != nil && ip.To4() != nil {
		cidr = ip.String() + "/32"
	} else {
		c, err := NormalizeIPv4PolicyCIDR(raw)
		if err != nil {
			return
		}
		_, n, err := net.ParseCIDR(c)
		if err != nil || n == nil || n.IP.To4() == nil {
			return
		}
		cidr = n.String()
	}
	if sessionLimitCIDRIgnored(cidr) {
		return
	}
	if _, ok := seen[cidr]; ok {
		return
	}
	seen[cidr] = struct{}{}
	*out = append(*out, cidr)
}

// vpnIPv4PolicyCIDRs 收集 ocserv / WireGuard 当前 IPv4 地址池（含组、vhost 与各 WG 实例）。
func vpnIPv4PolicyCIDRs(st State) []string {
	seen := map[string]struct{}{}
	var out []string
	appendIPv4PolicyCIDR(seen, &out, ocservPoolCIDR(st.VPN.OCServ))
	for _, g := range st.VPN.OCServ.Groups {
		appendIPv4PolicyCIDR(seen, &out, ipv4NetworkMaskCIDR(g.IPv4Network, g.IPv4Netmask))
	}
	for _, v := range st.VPN.OCServ.Vhosts {
		if !v.Enabled {
			continue
		}
		appendIPv4PolicyCIDR(seen, &out, ipv4NetworkMaskCIDR(v.IPv4Network, v.IPv4Netmask))
	}
	for _, w := range st.VPN.WireGuards {
		appendIPv4PolicyCIDR(seen, &out, strings.TrimSpace(w.Address))
	}
	return out
}

// RefreshVPNPolicyRoutes 按当前 VPN IPv4 池同步 auto_vpn_policy_routes；池变更时替换旧网段。
func RefreshVPNPolicyRoutes(st *State) {
	if st == nil {
		return
	}
	n := &st.Nat.IPv4
	if n.AutoPolicyRoutes == nil {
		n.AutoPolicyRoutes = []string{}
	}
	if n.AutoVPNPolicyRoutes == nil {
		n.AutoVPNPolicyRoutes = []string{}
	}
	needed := vpnIPv4PolicyCIDRs(*st)
	neededSet := map[string]struct{}{}
	for _, c := range needed {
		neededSet[c] = struct{}{}
	}
	var keptManual []string
	for _, c := range policyRouteManual(*n) {
		if _, isVPN := neededSet[c]; isVPN {
			continue
		}
		keptManual = append(keptManual, c)
	}
	var nextVPN []string
	for _, cidr := range needed {
		if CIDRCoveredByExisting(keptManual, cidr) || CIDRCoveredByExisting(n.AutoPolicyRoutes, cidr) || CIDRCoveredByExisting(nextVPN, cidr) {
			continue
		}
		nextVPN = append(nextVPN, cidr)
	}
	n.AutoVPNPolicyRoutes = nextVPN
	rebuildPolicyRoutes(n, keptManual)
}

func subtractPolicyRoutes(all, remove []string) []string {
	rm := map[string]struct{}{}
	for _, c := range remove {
		rm[strings.TrimSpace(c)] = struct{}{}
	}
	var out []string
	for _, c := range all {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, skip := rm[c]; skip {
			continue
		}
		out = append(out, c)
	}
	return out
}

// AddPolicyRouteManual 手工添加策略网段；已被现有网段覆盖时视为成功且不重复写入。
func AddPolicyRouteManual(n *NatIPv4State, cidr string) {
	if n == nil {
		return
	}
	if n.AutoPolicyRoutes == nil {
		n.AutoPolicyRoutes = []string{}
	}
	if n.AutoVPNPolicyRoutes == nil {
		n.AutoVPNPolicyRoutes = []string{}
	}
	cidr = strings.TrimSpace(cidr)
	if CIDRCoveredByExisting(n.PolicyRoutes, cidr) {
		n.PolicyRoutes = PruneContainedPolicyRoutes(n.PolicyRoutes)
		return
	}
	n.PolicyRoutes = PruneContainedPolicyRoutes(append(n.PolicyRoutes, cidr))
}

// RemovePolicyRouteManual 删除策略网段；若命中 auto 项则一并移除。
func RemovePolicyRouteManual(n *NatIPv4State, cidr string) {
	if n == nil {
		return
	}
	cidr = strings.TrimSpace(cidr)
	manual := policyRouteManual(*n)
	var keptManual, keptAuto, keptVPN []string
	for _, c := range manual {
		if c != cidr {
			keptManual = append(keptManual, c)
		}
	}
	for _, c := range n.AutoPolicyRoutes {
		if c != cidr {
			keptAuto = append(keptAuto, c)
		}
	}
	for _, c := range n.AutoVPNPolicyRoutes {
		if c != cidr {
			keptVPN = append(keptVPN, c)
		}
	}
	n.AutoPolicyRoutes = keptAuto
	n.AutoVPNPolicyRoutes = keptVPN
	rebuildPolicyRoutes(n, keptManual)
}
