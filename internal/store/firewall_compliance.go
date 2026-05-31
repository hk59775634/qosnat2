package store

import (
	"fmt"
	"strconv"
	"strings"
)

// AuditFilterRulesChange 审核待应用 filter 相对已应用规则的变化（不含 nft 内核校验）。
func AuditFilterRulesChange(applied, pending []FilterRule, aliases []AliasSet, adminPort, devLAN, devWAN string) ([]FirewallChangeIssue, FilterRulesDiff) {
	diff := diffFilterRules(applied, pending)
	var issues []FirewallChangeIssue
	appliedByID := map[string]FilterRule{}
	for _, r := range applied {
		appliedByID[r.ID] = r
	}
	for _, id := range append(append([]string{}, diff.Added...), diff.Modified...) {
		r, ok := findFilterRuleByID(pending, id)
		if !ok || !FilterRuleMutable(r) {
			continue
		}
		issues = append(issues, auditOneFilterRule(r, aliases, adminPort, devLAN, devWAN)...)
	}
	for _, id := range diff.Removed {
		if r, ok := appliedByID[id]; ok && FilterRuleMutable(r) {
			issues = append(issues, FirewallChangeIssue{
				Code:     "RULE_REMOVED",
				Severity: "warn",
				RuleID:   id,
				Message:  "removing a user filter rule",
				Hint:     "confirm the rule is no longer needed; traffic matching removed rules will follow later rules",
			})
		}
	}
	return issues, diff
}

func auditOneFilterRule(r FilterRule, aliases []AliasSet, adminPort, devLAN, devWAN string) []FirewallChangeIssue {
	var issues []FirewallChangeIssue
	if err := ValidateFilterRuleAliases(r, aliases); err != nil {
		issues = append(issues, FirewallChangeIssue{
			Code:     "ALIAS_UNKNOWN",
			Severity: "error",
			RuleID:   r.ID,
			Message:  err.Error(),
			Hint:     "create the alias under Firewall → Aliases, or use an explicit IP/CIDR instead of src_alias/dst_alias",
		})
	}
	if adminPort != "" && r.Enabled && r.Chain == "input" && r.Action == "accept" {
		if portMatchAdmin(r.DstPort, adminPort) && srcIsAny(r) && ifaceIsWAN(r.Iif, devWAN) {
			issues = append(issues, FirewallChangeIssue{
				Code:     "ADMIN_PORT_EXPOSED",
				Severity: "error",
				RuleID:   r.ID,
				Message:  fmt.Sprintf("accept on WAN input for admin port %s from any source", adminPort),
				Hint:     "restrict src_addr to management CIDR, use VPN, or rely on built-in admin allow rule only on LAN",
			})
		}
	}
	if r.Enabled && r.Chain == "input" && r.Action == "accept" && ifaceIsWAN(r.Iif, devWAN) && srcIsAny(r) && r.Proto == "" && r.DstPort == 0 {
		issues = append(issues, FirewallChangeIssue{
			Code:     "WAN_INPUT_BROAD_ACCEPT",
			Severity: "warn",
			RuleID:   r.ID,
			Message:  "broad accept on WAN input without protocol or port filter",
			Hint:     "prefer narrow rules (proto + dst_port + src CIDR); default WAN input should stay drop except VPN/admin",
		})
	}
	if r.Enabled && r.Action == "drop" && r.Chain == "forward" && (ifaceIsWAN(r.Iif, devWAN) || ifaceIsWAN(r.Oif, devWAN)) {
		issues = append(issues, FirewallChangeIssue{
			Code:     "WAN_BLOCK_WRONG_CHAIN",
			Severity: "warn",
			RuleID:   r.ID,
			Message:  "block/drop on forward chain does not filter WAN ingress",
			Hint:     "use input chain with iif set to WAN to block traffic arriving on the WAN interface",
		})
	}
	if r.Enabled && strings.TrimSpace(r.Comment) == "" {
		issues = append(issues, FirewallChangeIssue{
			Code:     "MISSING_COMMENT",
			Severity: "warn",
			RuleID:   r.ID,
			Message:  "rule has no description/comment",
			Hint:     "add a comment so operators can audit intent later",
		})
	}
	_ = devLAN
	return issues
}

func diffFilterRules(applied, pending []FilterRule) FilterRulesDiff {
	appliedMap := map[string]FilterRule{}
	pendingMap := map[string]FilterRule{}
	for _, r := range applied {
		if FilterRuleMutable(r) {
			appliedMap[r.ID] = r
		}
	}
	for _, r := range pending {
		if FilterRuleMutable(r) {
			pendingMap[r.ID] = r
		}
	}
	var diff FilterRulesDiff
	for id, r := range pendingMap {
		old, ok := appliedMap[id]
		if !ok {
			diff.Added = append(diff.Added, id)
			continue
		}
		if !filterRuleUserEqual(old, r) {
			diff.Modified = append(diff.Modified, id)
		}
	}
	for id := range appliedMap {
		if _, ok := pendingMap[id]; !ok {
			diff.Removed = append(diff.Removed, id)
		}
	}
	return diff
}

func filterRuleUserEqual(a, b FilterRule) bool {
	return FilterRulesEqual([]FilterRule{a}, []FilterRule{b})
}

func findFilterRuleByID(rules []FilterRule, id string) (FilterRule, bool) {
	for _, r := range rules {
		if r.ID == id {
			return r, true
		}
	}
	return FilterRule{}, false
}

func portMatchAdmin(rulePort int, adminPort string) bool {
	if rulePort == 0 {
		return false
	}
	p, err := strconv.Atoi(strings.TrimSpace(adminPort))
	return err == nil && p == rulePort
}

func srcIsAny(r FilterRule) bool {
	s := strings.TrimSpace(r.SrcAddr)
	return s == "" || s == "0.0.0.0/0" || s == "::/0"
}

func ifaceIsWAN(iface, devWAN string) bool {
	iface = strings.TrimSpace(iface)
	devWAN = strings.TrimSpace(devWAN)
	if iface == "" {
		return false
	}
	return iface == devWAN || strings.EqualFold(iface, "wan")
}

// ChangesHaveErrors 是否存在阻断应用的 error 级问题。
func ChangesHaveErrors(issues []FirewallChangeIssue) bool {
	for _, iss := range issues {
		if iss.Severity == "error" {
			return true
		}
	}
	return false
}
