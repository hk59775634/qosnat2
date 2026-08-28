package store

import "testing"

func TestCIDRCoveredByExisting(t *testing.T) {
	routes := []string{"10.0.0.0/8"}
	if !CIDRCoveredByExisting(routes, "10.22.0.3/32") {
		t.Fatal("10.22.0.3/32 should be covered by 10.0.0.0/8")
	}
	if CIDRCoveredByExisting(routes, "192.168.1.5/32") {
		t.Fatal("192.168.1.5/32 should not be covered by 10.0.0.0/8")
	}
}

func TestPruneContainedPolicyRoutes(t *testing.T) {
	in := []string{"10.0.0.0/8", "10.22.0.3/32", "10.250.0.0/24"}
	out := PruneContainedPolicyRoutes(in)
	if len(out) != 1 || out[0] != "10.0.0.0/8" {
		t.Fatalf("got %v", out)
	}
}

func TestRefreshMappingPolicyRoutesAddAndRemove(t *testing.T) {
	n := NatIPv4State{
		PolicyRoutes:   []string{"10.0.0.0/8"},
		AutoPolicyRoutes: []string{},
		StaticMappings: map[string]string{"10.22.0.3": "203.0.113.3"},
		PrefixMappings: map[string]string{},
	}
	if err := RefreshMappingPolicyRoutes(&n); err != nil {
		t.Fatal(err)
	}
	if len(n.AutoPolicyRoutes) != 0 {
		t.Fatalf("auto routes should stay empty when covered, got %v", n.AutoPolicyRoutes)
	}
	if len(n.PolicyRoutes) != 1 || n.PolicyRoutes[0] != "10.0.0.0/8" {
		t.Fatalf("policy routes = %v", n.PolicyRoutes)
	}

	n.StaticMappings["192.168.1.5"] = "203.0.113.5"
	if err := RefreshMappingPolicyRoutes(&n); err != nil {
		t.Fatal(err)
	}
	if len(n.AutoPolicyRoutes) != 1 || n.AutoPolicyRoutes[0] != "192.168.1.5/32" {
		t.Fatalf("auto routes = %v", n.AutoPolicyRoutes)
	}
	if !CIDRCoveredByExisting(n.PolicyRoutes, "192.168.1.5/32") {
		t.Fatalf("policy routes = %v", n.PolicyRoutes)
	}

	delete(n.StaticMappings, "192.168.1.5")
	if err := RefreshMappingPolicyRoutes(&n); err != nil {
		t.Fatal(err)
	}
	if len(n.AutoPolicyRoutes) != 0 {
		t.Fatalf("auto routes should be cleared, got %v", n.AutoPolicyRoutes)
	}
	if CIDRCoveredByExisting(n.PolicyRoutes, "192.168.1.5/32") {
		t.Fatalf("removed auto route should be gone from policy routes: %v", n.PolicyRoutes)
	}
}

func TestAddPolicyRouteManualSkipsContained(t *testing.T) {
	n := NatIPv4State{PolicyRoutes: []string{"10.0.0.0/8"}}
	AddPolicyRouteManual(&n, "10.22.0.3/32")
	if len(n.PolicyRoutes) != 1 {
		t.Fatalf("got %v", n.PolicyRoutes)
	}
}

func TestFilterPolicyRoutesForWANContained(t *testing.T) {
	in := []string{"10.0.0.0/8", "10.1.0.0/24"}
	out := FilterPolicyRoutesForWAN(in, []string{"10.0.0.0/8"})
	if len(out) != 0 {
		t.Fatalf("got %v", out)
	}
}
