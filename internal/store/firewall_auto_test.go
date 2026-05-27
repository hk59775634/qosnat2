package store

import "testing"

func TestSyncAutoFilterRules(t *testing.T) {
	vpn := AutoInputVPN{OCServEnabled: true, OCServTCP: 443, OCServUDP: 443, WGPorts: []int{51820}}
	user := FilterRule{ID: "fr-1", Chain: "forward", Action: "drop", Enabled: true}
	merged, changed := SyncAutoFilterRules([]FilterRule{user}, "eth0", "8443", vpn)
	if !changed {
		t.Fatal("expected change on first sync")
	}
	if len(merged) < 3 {
		t.Fatalf("expected user + auto rules, got %d", len(merged))
	}
	if merged[0].ID != "fr-1" {
		t.Fatal("user rule should stay first")
	}
	if !IsAutoManagedRule(merged[len(merged)-1]) {
		t.Fatal("last rule should be auto wan drop")
	}
	_, changed2 := SyncAutoFilterRules(merged, "eth0", "8443", vpn)
	if changed2 {
		t.Fatal("second sync should be no-op")
	}
}

func TestBuildAutoInputRulesAdminPort(t *testing.T) {
	rules := BuildAutoInputRules("wan0", "9090", AutoInputVPN{})
	if len(rules) < 2 {
		t.Fatal("expected admin + wan drop")
	}
	if rules[0].DstPort != 9090 {
		t.Fatalf("admin port: got %d", rules[0].DstPort)
	}
}
