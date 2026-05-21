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
	r.Proto = strings.ToLower(strings.TrimSpace(r.Proto))
	r.SrcAddr = strings.TrimSpace(r.SrcAddr)
	r.DstAddr = strings.TrimSpace(r.DstAddr)
	r.Comment = strings.TrimSpace(r.Comment)
	return nil
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
