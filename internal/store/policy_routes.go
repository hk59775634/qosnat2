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

// RefreshMappingPolicyRoutes 根据 1:1 / 网段映射同步 auto_policy_routes，并清理冗余策略网段。
func RefreshMappingPolicyRoutes(n *NatIPv4State) error {
	if n == nil {
		return nil
	}
	if n.AutoPolicyRoutes == nil {
		n.AutoPolicyRoutes = []string{}
	}
	needed, err := mappingPolicyCIDRs(*n)
	if err != nil {
		return err
	}
	manual := subtractPolicyRoutes(n.PolicyRoutes, n.AutoPolicyRoutes)
	var nextAuto []string
	for _, cidr := range needed {
		if CIDRCoveredByExisting(manual, cidr) || CIDRCoveredByExisting(nextAuto, cidr) {
			continue
		}
		nextAuto = append(nextAuto, cidr)
	}
	n.AutoPolicyRoutes = nextAuto
	n.PolicyRoutes = PruneContainedPolicyRoutes(append(append([]string(nil), manual...), nextAuto...))
	return nil
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
	manual := subtractPolicyRoutes(n.PolicyRoutes, n.AutoPolicyRoutes)
	var keptManual, keptAuto []string
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
	n.AutoPolicyRoutes = keptAuto
	n.PolicyRoutes = PruneContainedPolicyRoutes(append(keptManual, keptAuto...))
}
