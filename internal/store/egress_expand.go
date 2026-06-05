package store

import "strings"

// EgressIPRule 一条 ip rule（from/to 空表示省略该维度）。
type EgressIPRule struct {
	From     string
	To       string
	Table    int
	Priority int
	Mode     string // source|destination|both — 决定 main 旁路规则形态
}

// ExpandEgressIPRules 将出站策略展开为 ip rule 列表（别名展开为多 CIDR）。
func ExpandEgressIPRules(p EgressPolicy, table int, aliases map[string]AliasSet) ([]EgressIPRule, error) {
	srcs, err := AliasMembers(p.SrcCIDR, p.SrcAlias, aliases)
	if err != nil {
		return nil, err
	}
	dsts, err := AliasMembers(p.DstCIDR, p.DstAlias, aliases)
	if err != nil {
		return nil, err
	}
	// legacy fallback when normalize not applied
	if len(srcs) == 0 && len(dsts) == 0 && p.CIDR != "" {
		if p.Match == "destination" {
			dsts = []string{p.CIDR}
		} else {
			srcs = []string{p.CIDR}
		}
	}
	if len(srcs) == 0 {
		srcs = []string{""}
	}
	if len(dsts) == 0 {
		dsts = []string{""}
	}
	mode := "both"
	if srcs[0] == "" && len(srcs) == 1 {
		mode = "destination"
	} else if dsts[0] == "" && len(dsts) == 1 {
		mode = "source"
	}
	var rules []EgressIPRule
	for _, src := range srcs {
		for _, dst := range dsts {
			rules = append(rules, EgressIPRule{
				From:     src,
				To:       dst,
				Table:    table,
				Priority: p.Priority,
				Mode:     mode,
			})
		}
	}
	return rules, nil
}

// EgressEndpointsLabel 用于 UI/路由说明。
func EgressEndpointsLabel(p EgressPolicy) string {
	var parts []string
	if p.SrcAlias != "" {
		parts = append(parts, "src:@"+p.SrcAlias)
	} else if p.SrcCIDR != "" {
		parts = append(parts, "src:"+p.SrcCIDR)
	}
	if p.DstAlias != "" {
		parts = append(parts, "dst:@"+p.DstAlias)
	} else if p.DstCIDR != "" {
		parts = append(parts, "dst:"+p.DstCIDR)
	}
	if len(parts) == 0 && p.CIDR != "" {
		if p.Match == "destination" {
			parts = append(parts, "dst:"+p.CIDR)
		} else {
			parts = append(parts, "src:"+p.CIDR)
		}
	}
	return strings.Join(parts, " ")
}
