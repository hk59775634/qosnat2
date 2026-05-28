package store

import "testing"

func TestReorderFirewallRulesSkipsAutoIDs(t *testing.T) {
	rules := []FilterRule{
		{ID: "fr-1", Chain: "forward", Action: "drop", Enabled: true},
		{ID: "fr-2", Chain: "forward", Action: "accept", Enabled: true},
		{
			ID: autoIDInputAdmin + "-eth0", Chain: "input", Action: "accept",
			Iif: "eth0", Enabled: true, System: true,
		},
		{ID: autoIDInputWanDrop + "-eth0", Chain: "input", Action: "drop", Iif: "eth0", Enabled: true, System: true},
	}
	order := []string{"fr-2", "auto-input-admin-eth0", "fr-1", "auto-input-wan-drop-eth0"}
	out, err := ReorderFirewallRules(rules, order)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 4 {
		t.Fatalf("len %d", len(out))
	}
	if out[0].ID != "fr-2" || out[1].ID != "fr-1" {
		t.Fatalf("user order: %v %v", out[0].ID, out[1].ID)
	}
	if !IsAutoManagedRule(out[2]) || !IsAutoManagedRule(out[3]) {
		t.Fatalf("auto rules should trail: %v %v", out[2].ID, out[3].ID)
	}
}

func TestReorderFirewallRulesMutableOnly(t *testing.T) {
	rules := []FilterRule{
		{ID: "fr-a", Chain: "input", Action: "accept", Enabled: true},
		{ID: "fr-b", Chain: "input", Action: "drop", Enabled: true},
	}
	out, err := ReorderFirewallRules(rules, []string{"fr-b", "fr-a"})
	if err != nil {
		t.Fatal(err)
	}
	if out[0].ID != "fr-b" || out[1].ID != "fr-a" {
		t.Fatalf("got %s %s", out[0].ID, out[1].ID)
	}
}
