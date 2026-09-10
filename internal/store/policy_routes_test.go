package store

import (
	"path/filepath"
	"testing"

	"github.com/hk59775634/qosnat2/internal/linknet"
)

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
		PolicyRoutes:     []string{"10.0.0.0/8"},
		AutoPolicyRoutes: []string{},
		StaticMappings:   map[string]string{"10.22.0.3": "203.0.113.3"},
		PrefixMappings:   map[string]string{},
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

func TestRefreshMappingPolicyRoutesKeepsVPNAutos(t *testing.T) {
	n := NatIPv4State{
		PolicyRoutes:        []string{"10.0.0.0/8", "198.18.250.0/24"},
		AutoPolicyRoutes:    []string{},
		AutoVPNPolicyRoutes: []string{"198.18.250.0/24"},
		StaticMappings:      map[string]string{},
		PrefixMappings:      map[string]string{},
	}
	if err := RefreshMappingPolicyRoutes(&n); err != nil {
		t.Fatal(err)
	}
	if !CIDRCoveredByExisting(n.PolicyRoutes, "198.18.250.0/24") {
		t.Fatalf("VPN auto CIDR dropped by mapping refresh: %v", n.PolicyRoutes)
	}
	if len(n.AutoVPNPolicyRoutes) != 1 || n.AutoVPNPolicyRoutes[0] != "198.18.250.0/24" {
		t.Fatalf("auto vpn routes = %v", n.AutoVPNPolicyRoutes)
	}
}

func TestAddPolicyRouteManualSkipsContained(t *testing.T) {
	n := NatIPv4State{PolicyRoutes: []string{"10.0.0.0/8"}}
	AddPolicyRouteManual(&n, "10.22.0.3/32")
	if len(n.PolicyRoutes) != 1 {
		t.Fatalf("got %v", n.PolicyRoutes)
	}
}

func TestRefreshVPNPolicyRoutesDefaultPools(t *testing.T) {
	st := DefaultState()
	RefreshVPNPolicyRoutes(&st)
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, linknet.OCServDefaultIPv4CIDR) {
		t.Fatalf("missing ocserv default pool in %v", st.Nat.IPv4.PolicyRoutes)
	}
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, "198.19.0.0/24") {
		t.Fatalf("missing wireguard default pool in %v", st.Nat.IPv4.PolicyRoutes)
	}
	if !containsCIDR(st.Nat.IPv4.AutoVPNPolicyRoutes, linknet.OCServDefaultIPv4CIDR) {
		t.Fatalf("ocserv pool should be auto-vpn, got %v", st.Nat.IPv4.AutoVPNPolicyRoutes)
	}
	if !containsCIDR(st.Nat.IPv4.AutoVPNPolicyRoutes, "198.19.0.0/24") {
		t.Fatalf("wg pool should be auto-vpn, got %v", st.Nat.IPv4.AutoVPNPolicyRoutes)
	}
}

func TestRefreshVPNPolicyRoutesUpdatesOnPoolChange(t *testing.T) {
	st := DefaultState()
	st.Nat.IPv4.PolicyRoutes = []string{"10.0.0.0/8"}
	RefreshVPNPolicyRoutes(&st)
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, linknet.OCServDefaultIPv4CIDR) {
		t.Fatalf("want default ocserv pool, got %v", st.Nat.IPv4.PolicyRoutes)
	}

	st.VPN.OCServ.IPv4Network = "198.18.251.0"
	st.VPN.OCServ.IPv4Netmask = "255.255.255.0"
	RefreshVPNPolicyRoutes(&st)
	if CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, linknet.OCServDefaultIPv4CIDR) {
		t.Fatalf("old ocserv pool should be removed: %v", st.Nat.IPv4.PolicyRoutes)
	}
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, "198.18.251.0/24") {
		t.Fatalf("new ocserv pool missing: %v", st.Nat.IPv4.PolicyRoutes)
	}
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, "10.0.0.0/8") {
		t.Fatalf("manual CIDR should stay: %v", st.Nat.IPv4.PolicyRoutes)
	}

	st.VPN.WireGuards[0].Address = "198.19.8.1/24"
	RefreshVPNPolicyRoutes(&st)
	if CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, "198.19.0.0/24") {
		t.Fatalf("old wg pool should be removed: %v", st.Nat.IPv4.PolicyRoutes)
	}
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, "198.19.8.0/24") {
		t.Fatalf("new wg pool missing: %v", st.Nat.IPv4.PolicyRoutes)
	}
}

func TestRefreshVPNPolicyRoutesSkipsWhenCovered(t *testing.T) {
	st := DefaultState()
	st.Nat.IPv4.PolicyRoutes = []string{"10.0.0.0/8"}
	st.VPN.OCServ.IPv4Network = "10.8.0.0"
	st.VPN.OCServ.IPv4Netmask = "255.255.255.0"
	RefreshVPNPolicyRoutes(&st)
	if containsCIDR(st.Nat.IPv4.PolicyRoutes, "10.8.0.0/24") {
		t.Fatalf("covered ocserv pool should not be duplicated: %v", st.Nat.IPv4.PolicyRoutes)
	}
	if containsCIDR(st.Nat.IPv4.AutoVPNPolicyRoutes, "10.8.0.0/24") {
		t.Fatalf("covered pool should not be auto-vpn: %v", st.Nat.IPv4.AutoVPNPolicyRoutes)
	}
}

func TestRefreshVPNPolicyRoutesGroupAndVhost(t *testing.T) {
	st := DefaultState()
	st.VPN.OCServ.Groups = []OCServGroup{{
		Name:        "g1",
		IPv4Network: "198.18.252.0",
		IPv4Netmask: "255.255.255.0",
	}}
	st.VPN.OCServ.Vhosts = []OCServVhost{{
		Enabled:     true,
		Domain:      "vpn.example",
		IPv4Network: "198.18.253.0",
		IPv4Netmask: "255.255.255.0",
	}}
	RefreshVPNPolicyRoutes(&st)
	for _, cidr := range []string{"198.18.252.0/24", "198.18.253.0/24"} {
		if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, cidr) {
			t.Fatalf("missing %s in %v", cidr, st.Nat.IPv4.PolicyRoutes)
		}
	}
	st.VPN.OCServ.Vhosts[0].Enabled = false
	RefreshVPNPolicyRoutes(&st)
	if containsCIDR(st.Nat.IPv4.AutoVPNPolicyRoutes, "198.18.253.0/24") {
		t.Fatalf("disabled vhost pool should be removed: %v", st.Nat.IPv4.AutoVPNPolicyRoutes)
	}
}

func TestEnsureDefaultsLockedSyncsVPNPolicyRoutes(t *testing.T) {
	s := New(filepath.Join(t.TempDir(), "state.json"))
	if err := s.Update(func(*State) {}); err != nil {
		t.Fatal(err)
	}
	st := s.Get()
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, linknet.OCServDefaultIPv4CIDR) {
		t.Fatalf("load/update should write ocserv default pool, got %v", st.Nat.IPv4.PolicyRoutes)
	}
	if !CIDRCoveredByExisting(st.Nat.IPv4.PolicyRoutes, "198.19.0.0/24") {
		t.Fatalf("load/update should write wg default pool, got %v", st.Nat.IPv4.PolicyRoutes)
	}
}

func containsCIDR(routes []string, cidr string) bool {
	for _, r := range routes {
		if r == cidr {
			return true
		}
	}
	return false
}

func TestFilterPolicyRoutesForWANContained(t *testing.T) {
	in := []string{"10.0.0.0/8", "10.1.0.0/24"}
	out := FilterPolicyRoutesForWAN(in, []string{"10.0.0.0/8"})
	if len(out) != 0 {
		t.Fatalf("got %v", out)
	}
}
