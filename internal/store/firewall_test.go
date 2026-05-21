package store

import "testing"

func TestFilterRuleNftLine(t *testing.T) {
	r := FilterRule{
		Chain: "forward", Action: "drop", Iif: "ens18", Oif: "ens19",
		SrcAddr: "203.0.113.0/24", DstPort: 443, Proto: "tcp", Enabled: true,
	}
	line := r.NftRuleLine()
	if line == "" {
		t.Fatal("empty line")
	}
	if !containsAll(line, "iifname", "drop", "tcp", "dport 443") {
		t.Fatalf("unexpected: %s", line)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
