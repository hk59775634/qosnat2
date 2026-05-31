package store

import "encoding/json"

// CloneFilterRules 深拷贝 filter 规则列表。
func CloneFilterRules(rules []FilterRule) []FilterRule {
	return append([]FilterRule(nil), rules...)
}

// FilterRulesEqual 比较两条规则列表是否一致（含顺序）。
func FilterRulesEqual(a, b []FilterRule) bool {
	ab, err := json.Marshal(a)
	if err != nil {
		return false
	}
	bb, err := json.Marshal(b)
	if err != nil {
		return false
	}
	return string(ab) == string(bb)
}

// FirewallChangeIssue 待应用变更的合规/预检问题。
type FirewallChangeIssue struct {
	Code     string `json:"code"`
	Severity string `json:"severity"` // error | warn
	RuleID   string `json:"rule_id,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint"`
}

// FilterRulesDiff 已应用 vs 待应用规则差异摘要。
type FilterRulesDiff struct {
	Added    []string `json:"added,omitempty"`
	Removed  []string `json:"removed,omitempty"`
	Modified []string `json:"modified,omitempty"`
}
