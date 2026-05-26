package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
)

// FilterRule 用户自定义 nft filter 规则
type FilterRule struct {
	ID      string `json:"id"`
	Chain   string `json:"chain"`   // forward | input
	Action  string `json:"action"`  // accept | drop | reject
	Iif     string `json:"iif,omitempty"`
	Oif     string `json:"oif,omitempty"`
	Proto   string `json:"proto,omitempty"`
	SrcAddr   string `json:"src_addr,omitempty"`
	DstAddr   string `json:"dst_addr,omitempty"`
	SrcAlias  string `json:"src_alias,omitempty"`
	DstAlias  string `json:"dst_alias,omitempty"`
	SrcPort int    `json:"src_port,omitempty"`
	DstPort int    `json:"dst_port,omitempty"`
	Comment string `json:"comment,omitempty"`
	Enabled bool   `json:"enabled"`
	// System 为 true 时表示平台内置/受管规则，禁止通过 API 修改或删除。
	System bool `json:"system,omitempty"`
}

// FilterRuleMutable 是否允许用户修改或删除（系统规则及保留 ID 前缀不可变）。
func FilterRuleMutable(r FilterRule) bool {
	if r.System {
		return false
	}
	id := strings.TrimSpace(r.ID)
	if id == "" {
		return false
	}
	if strings.HasPrefix(id, "sys-") || strings.HasPrefix(id, "auto-") {
		return false
	}
	return true
}

// NormalizeFilterRule 校验并填充默认值
func NormalizeFilterRule(r *FilterRule) error {
	if r == nil {
		return fmt.Errorf("rule nil")
	}
	if r.ID == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		r.ID = "fr-" + hex.EncodeToString(b)
	} else if err := validateFilterRuleID(r.ID); err != nil {
		return err
	}
	chain := strings.ToLower(strings.TrimSpace(r.Chain))
	if chain != "forward" && chain != "input" {
		return fmt.Errorf("chain must be forward or input")
	}
	r.Chain = chain
	act := strings.ToLower(strings.TrimSpace(r.Action))
	switch act {
	case "accept", "drop", "reject":
		r.Action = act
	default:
		return fmt.Errorf("action must be accept, drop, or reject")
	}
	r.Iif = strings.TrimSpace(r.Iif)
	r.Oif = strings.TrimSpace(r.Oif)
	if chain == "input" {
		r.Oif = ""
	}
	if r.Iif != "" {
		if err := ValidateIfaceName(r.Iif); err != nil {
			return fmt.Errorf("iif: %w", err)
		}
	}
	if r.Oif != "" {
		if err := ValidateIfaceName(r.Oif); err != nil {
			return fmt.Errorf("oif: %w", err)
		}
	}
	r.Proto = strings.ToLower(strings.TrimSpace(r.Proto))
	if r.Proto != "" && r.Proto != "all" {
		switch r.Proto {
		case "tcp", "udp", "icmp", "icmpv6", "sctp", "udplite":
		default:
			return fmt.Errorf("unsupported proto %q", r.Proto)
		}
	}
	r.SrcAddr = strings.TrimSpace(r.SrcAddr)
	r.DstAddr = strings.TrimSpace(r.DstAddr)
	if r.SrcAddr != "" {
		if err := ValidateIPv4OrCIDR(r.SrcAddr); err != nil {
			return fmt.Errorf("src_addr: %w", err)
		}
	}
	if r.DstAddr != "" {
		if err := ValidateIPv4OrCIDR(r.DstAddr); err != nil {
			return fmt.Errorf("dst_addr: %w", err)
		}
	}
	r.SrcAlias = strings.TrimSpace(r.SrcAlias)
	r.DstAlias = strings.TrimSpace(r.DstAlias)
	if r.SrcAlias != "" {
		if err := ValidateAliasName(r.SrcAlias); err != nil {
			return fmt.Errorf("src_alias: %w", err)
		}
	}
	if r.DstAlias != "" {
		if err := ValidateAliasName(r.DstAlias); err != nil {
			return fmt.Errorf("dst_alias: %w", err)
		}
	}
	if strings.ContainsAny(r.Comment, "\n\r") {
		return fmt.Errorf("comment must not contain newlines")
	}
	r.Comment = strings.TrimSpace(r.Comment)
	return nil
}

func validateFilterRuleID(id string) error {
	id = strings.TrimSpace(id)
	if strings.HasPrefix(id, "sys-") || strings.HasPrefix(id, "auto-") {
		return fmt.Errorf("rule id prefix reserved for system rules")
	}
	return nil
}

// RepairFilterRuleIDs 为缺少 id 的历史规则补全 ID（返回是否有修复）。
func RepairFilterRuleIDs(rules []FilterRule) ([]FilterRule, bool) {
	changed := false
	out := make([]FilterRule, len(rules))
	for i, r := range rules {
		if strings.TrimSpace(r.ID) == "" {
			_ = NormalizeFilterRule(&r)
			changed = true
		}
		out[i] = r
	}
	return out, changed
}

// NftRuleLine 生成单行 nft 规则（不含前导空格）
func (r FilterRule) NftRuleLine() string {
	if !r.Enabled {
		return ""
	}
	var parts []string
	if r.Iif != "" {
		parts = append(parts, fmt.Sprintf(`iifname "%s"`, r.Iif))
	}
	if r.Oif != "" {
		parts = append(parts, fmt.Sprintf(`oifname "%s"`, r.Oif))
	}
	if r.Proto != "" && r.Proto != "all" {
		parts = append(parts, r.Proto)
	}
	if r.SrcAlias != "" {
		parts = append(parts, "ip saddr @alias_"+r.SrcAlias)
	} else if r.SrcAddr != "" {
		parts = append(parts, "ip saddr "+r.SrcAddr)
	}
	if r.DstAlias != "" {
		parts = append(parts, "ip daddr @alias_"+r.DstAlias)
	} else if r.DstAddr != "" {
		parts = append(parts, "ip daddr "+r.DstAddr)
	}
	if r.SrcPort > 0 {
		parts = append(parts, fmt.Sprintf("sport %d", r.SrcPort))
	}
	if r.DstPort > 0 {
		parts = append(parts, fmt.Sprintf("dport %d", r.DstPort))
	}
	parts = append(parts, r.Action)
	return strings.Join(parts, " ")
}
