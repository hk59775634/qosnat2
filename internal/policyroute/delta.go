package policyroute

import (
	"sort"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

// Delta 描述两次出站策略列表之间需要同步的数据面。
type Delta struct {
	IPRules bool
	NFT     bool
}

func policiesByID(list []store.EgressPolicy) map[string]store.EgressPolicy {
	out := make(map[string]store.EgressPolicy, len(list))
	for _, p := range list {
		out[p.ID] = p
	}
	return out
}

func egressIPRuleEqual(a, b store.EgressPolicy) bool {
	return a.Enabled == b.Enabled &&
		a.WanLinkID == b.WanLinkID &&
		a.Priority == b.Priority &&
		a.SrcCIDR == b.SrcCIDR &&
		a.SrcAlias == b.SrcAlias &&
		a.SrcIface == b.SrcIface &&
		a.DstCIDR == b.DstCIDR &&
		a.DstAlias == b.DstAlias &&
		a.CIDR == b.CIDR &&
		a.Match == b.Match
}

func egressNftEqual(a, b store.EgressPolicy) bool {
	return a.Enabled == b.Enabled &&
		a.WanLinkID == b.WanLinkID &&
		a.SNATIP == b.SNATIP &&
		a.NoSNAT == b.NoSNAT &&
		a.SrcCIDR == b.SrcCIDR &&
		a.SrcAlias == b.SrcAlias &&
		a.SrcIface == b.SrcIface &&
		a.DstCIDR == b.DstCIDR &&
		a.DstAlias == b.DstAlias &&
		a.CIDR == b.CIDR &&
		a.Match == b.Match
}

// PlanDelta 判断 prev→next 是否需要改 ip rule / nft SNAT。
func PlanDelta(prev, next []store.EgressPolicy) Delta {
	var d Delta
	prevBy := policiesByID(prev)
	nextBy := policiesByID(next)
	for id, p := range prevBy {
		np, ok := nextBy[id]
		if !ok {
			d.IPRules = true
			d.NFT = true
			continue
		}
		if !egressIPRuleEqual(p, np) {
			d.IPRules = true
		}
		if !egressNftEqual(p, np) {
			d.NFT = true
		}
	}
	for id := range nextBy {
		if _, ok := prevBy[id]; !ok {
			d.IPRules = true
			d.NFT = true
		}
	}
	return d
}

// ReferencedWansChanged 已启用策略引用的 WanLink 集合是否变化（影响策略表 default 路由）。
func ReferencedWansChanged(prev, next []store.EgressPolicy) bool {
	return referencedWanKey(prev) != referencedWanKey(next)
}

func referencedWanKey(list []store.EgressPolicy) string {
	seen := map[string]struct{}{}
	var ids []string
	for _, p := range list {
		if !p.Enabled {
			continue
		}
		id := p.WanLinkID
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Strings(ids)
	out := ""
	for i, id := range ids {
		if i > 0 {
			out += ","
		}
		out += id
	}
	return out
}

// ApplyDelta 只同步 prev→next 中有变化的策略 ip rule，不重放未改动的策略。
func ApplyDelta(st store.State, prev, next []store.EgressPolicy) error {
	aliases := store.AliasByName(st.Firewall.Aliases)
	resolved := store.ResolveEgressPolicies(st, netif.PrimaryIPv4)
	resolvedIDs := make(map[string]struct{}, len(resolved))
	for _, re := range resolved {
		resolvedIDs[re.Policy.ID] = struct{}{}
	}
	prevBy := policiesByID(prev)
	nextBy := policiesByID(next)

	if err := checkUnresolvedEgress(st, resolved); err != nil {
		return err
	}

	for id, p := range prevBy {
		np, ok := nextBy[id]
		if ok && egressIPRuleEqual(p, np) {
			continue
		}
		DeletePolicy(p, st.Network.WanLinks, aliases)
	}
	for id, p := range nextBy {
		op, ok := prevBy[id]
		if ok && egressIPRuleEqual(op, p) {
			continue
		}
		if !p.Enabled {
			continue
		}
		if _, ok := resolvedIDs[p.ID]; !ok {
			continue
		}
		if err := addPolicyRules(p, st, aliases); err != nil {
			return err
		}
	}
	flushRouteCache()
	return nil
}

func addPolicyRules(p store.EgressPolicy, st store.State, aliases map[string]store.AliasSet) error {
	tbl := store.WanLinkRouteTable(p.WanLinkID, st.Network.WanLinks)
	if tbl <= 0 {
		return nil
	}
	rules, err := store.ExpandEgressIPRules(p, tbl, aliases)
	if err != nil {
		return err
	}
	for _, r := range rules {
		if err := addExpandedRule(r); err != nil {
			return err
		}
	}
	return nil
}

const maxFlushCIDRs = 64

// ChangedCIDRs 收集变更策略的源/目的 CIDR，供按需清理 conntrack。
func ChangedCIDRs(st store.State, prev, next []store.EgressPolicy) []string {
	aliases := store.AliasByName(st.Firewall.Aliases)
	prevBy := policiesByID(prev)
	nextBy := policiesByID(next)
	seen := map[string]struct{}{}
	var out []string
	addPolicy := func(p store.EgressPolicy) {
		for _, c := range policyCIDRs(p, aliases) {
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
			if len(out) >= maxFlushCIDRs {
				return
			}
		}
	}
	for id, p := range prevBy {
		np, ok := nextBy[id]
		if ok && egressIPRuleEqual(p, np) && egressNftEqual(p, np) {
			continue
		}
		addPolicy(p)
		if ok {
			addPolicy(np)
		}
		if len(out) >= maxFlushCIDRs {
			return out
		}
	}
	for id, p := range nextBy {
		if _, ok := prevBy[id]; ok {
			continue
		}
		addPolicy(p)
		if len(out) >= maxFlushCIDRs {
			return out
		}
	}
	return out
}

func policyCIDRs(p store.EgressPolicy, aliases map[string]store.AliasSet) []string {
	var out []string
	appendMembers := func(cidr, alias string) {
		members, err := store.AliasMembers(cidr, alias, aliases)
		if err != nil {
			if cidr != "" {
				out = append(out, cidr)
			}
			return
		}
		out = append(out, members...)
	}
	appendMembers(p.SrcCIDR, p.SrcAlias)
	appendMembers(p.DstCIDR, p.DstAlias)
	if p.CIDR != "" && p.SrcCIDR == "" && p.DstCIDR == "" && p.SrcAlias == "" && p.DstAlias == "" {
		out = append(out, p.CIDR)
	}
	return out
}
