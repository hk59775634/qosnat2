package store

import (
	"fmt"
	"strings"
)

const firewallGatewayEgressPrefix = "auto-fw-gw-"

// SyncFirewallGatewayEgress 将带 wan_link_id 的 accept 规则同步为策略路由出站条目。
// 不使用 skb mark（遵守 mark 隔离）；复用现有 EgressPolicy + ip rule。
func SyncFirewallGatewayEgress(st *State) bool {
	if st == nil {
		return false
	}
	keep := make([]EgressPolicy, 0, len(st.Network.EgressPolicies))
	changed := false
	for _, p := range st.Network.EgressPolicies {
		if strings.HasPrefix(p.ID, firewallGatewayEgressPrefix) {
			changed = true
			continue
		}
		keep = append(keep, p)
	}
	aliasBy := AliasByName(st.Firewall.Aliases)
	for _, r := range st.Firewall.FilterRules {
		if !FilterRuleMutable(r) || !r.Enabled {
			continue
		}
		wanID := strings.TrimSpace(r.WanLinkID)
		if wanID == "" {
			continue
		}
		act := strings.ToLower(strings.TrimSpace(r.Action))
		if act != "accept" {
			continue
		}
		if strings.ToLower(strings.TrimSpace(r.Chain)) != "forward" {
			continue
		}
		if _, ok := FindWanLink(st.Network.WanLinks, wanID); !ok {
			continue
		}
		p := EgressPolicy{
			ID:        firewallGatewayEgressPrefix + r.ID,
			Name:      "fw:" + r.ID,
			SrcCIDR:   strings.TrimSpace(r.SrcAddr),
			DstCIDR:   strings.TrimSpace(r.DstAddr),
			SrcAlias:  strings.TrimSpace(r.SrcAlias),
			DstAlias:  strings.TrimSpace(r.DstAlias),
			SrcIface:  strings.TrimSpace(r.Iif),
			WanLinkID: wanID,
			Priority:  50,
			Enabled:   true,
		}
		// 别名存在性：无效则跳过该条，避免坏 ip rule
		if p.SrcAlias != "" {
			if _, ok := aliasBy[p.SrcAlias]; !ok {
				continue
			}
		}
		if p.DstAlias != "" {
			if _, ok := aliasBy[p.DstAlias]; !ok {
				continue
			}
		}
		if p.SrcCIDR == "" && p.DstCIDR == "" && p.SrcAlias == "" && p.DstAlias == "" && p.SrcIface == "" {
			continue
		}
		if err := NormalizeEgressPolicy(&p); err != nil {
			continue
		}
		keep = append(keep, p)
		changed = true
	}
	st.Network.EgressPolicies = keep
	return changed
}

// FirewallShaperTenantPrefix 规则绑定 QoS 时写入 profile 的 tenant_id 前缀。
const FirewallShaperTenantPrefix = "fw:"

// SyncFirewallShaperProfiles 将规则上的 shaper_down/up + 源匹配同步为 profile_lpm。
// 仅追加/替换 tenant_id=fw:<ruleid> 的条目，不影响手工/租户 profiles。
func SyncFirewallShaperProfiles(st *State) bool {
	if st == nil {
		return false
	}
	keep := make([]ProfileEntry, 0, len(st.Shaper.Profiles))
	changed := false
	for _, p := range st.Shaper.Profiles {
		if strings.HasPrefix(p.TenantID, FirewallShaperTenantPrefix) {
			changed = true
			continue
		}
		keep = append(keep, p)
	}
	aliasBy := AliasByName(st.Firewall.Aliases)
	nextID := 1
	for _, p := range keep {
		if p.ID >= nextID {
			nextID = p.ID + 1
		}
	}
	for _, r := range st.Firewall.FilterRules {
		if !FilterRuleMutable(r) || !r.Enabled {
			continue
		}
		down := strings.TrimSpace(r.ShaperDown)
		up := strings.TrimSpace(r.ShaperUp)
		if down == "" && up == "" {
			continue
		}
		cidrs := firewallRuleSourceCIDRs(r, aliasBy)
		if len(cidrs) == 0 {
			continue
		}
		tid := FirewallShaperTenantPrefix + r.ID
		for _, cidr := range cidrs {
			keep = append(keep, ProfileEntry{
				CIDR:     cidr,
				Down:     down,
				Up:       up,
				Mask:     32,
				ID:       nextID,
				TenantID: tid,
			})
			nextID++
			changed = true
		}
	}
	st.Shaper.Profiles = keep
	return changed
}

func firewallRuleSourceCIDRs(r FilterRule, aliasBy map[string]AliasSet) []string {
	if a := strings.TrimSpace(r.SrcAddr); a != "" {
		return []string{a}
	}
	name := strings.TrimSpace(r.SrcAlias)
	if name == "" {
		return nil
	}
	al, ok := aliasBy[name]
	if !ok {
		return nil
	}
	typ := strings.ToLower(strings.TrimSpace(al.Type))
	if typ == "port" {
		return nil
	}
	out := make([]string, 0, len(al.Members))
	for _, m := range al.Members {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		out = append(out, m)
	}
	return out
}

// ValidateFilterRulePortAliases 校验端口别名引用。
func ValidateFilterRulePortAliases(r FilterRule, aliases []AliasSet) error {
	byName := AliasByName(aliases)
	check := func(field, name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		a, ok := byName[name]
		if !ok {
			return fmt.Errorf("%s: alias %q not found", field, name)
		}
		if strings.ToLower(strings.TrimSpace(a.Type)) != "port" {
			return fmt.Errorf("%s: alias %q must be type port", field, name)
		}
		return nil
	}
	if err := check("src_port_alias", r.SrcPortAlias); err != nil {
		return err
	}
	return check("dst_port_alias", r.DstPortAlias)
}
