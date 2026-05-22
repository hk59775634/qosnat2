package store

import "fmt"

// ReorderFirewallRules 按 id 列表重排 filter 规则
func ReorderFirewallRules(rules []FilterRule, order []string) ([]FilterRule, error) {
	if len(order) == 0 {
		return nil, fmt.Errorf("order required")
	}
	byID := map[string]FilterRule{}
	for _, r := range rules {
		byID[r.ID] = r
	}
	out := make([]FilterRule, 0, len(rules))
	seen := map[string]struct{}{}
	for _, id := range order {
		r, ok := byID[id]
		if !ok {
			return nil, fmt.Errorf("unknown rule id %q", id)
		}
		out = append(out, r)
		seen[id] = struct{}{}
	}
	for _, r := range rules {
		if _, ok := seen[r.ID]; !ok {
			out = append(out, r)
		}
	}
	return out, nil
}
