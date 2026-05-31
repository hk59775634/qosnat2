package store

import "testing"

func TestAuditFilterRulesChangeAdminPort(t *testing.T) {
	applied := []FilterRule{}
	pending := []FilterRule{{
		ID:      "fr-test",
		Chain:   "input",
		Action:  "accept",
		Iif:     "eth1",
		DstPort: 8080,
		Enabled: true,
	}}
	issues, diff := AuditFilterRulesChange(applied, pending, nil, "8080", "br-lan", "eth1")
	if len(diff.Added) != 1 {
		t.Fatalf("diff added: %+v", diff)
	}
	found := false
	for _, iss := range issues {
		if iss.Code == "ADMIN_PORT_EXPOSED" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected ADMIN_PORT_EXPOSED, got %+v", issues)
	}
}

func TestChangesHaveErrors(t *testing.T) {
	if ChangesHaveErrors([]FirewallChangeIssue{{Severity: "warn"}}) {
		t.Fatal("warn should not block")
	}
	if !ChangesHaveErrors([]FirewallChangeIssue{{Severity: "error"}}) {
		t.Fatal("error should block")
	}
}
