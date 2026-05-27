package store

import (
	"encoding/json"
	"testing"
)

func TestMigrateNatFromLegacy(t *testing.T) {
	st := State{}
	leg := natLegacyFields{
		PolicyRoutes:   []string{"10.1.0.0/16"},
		SharedIPs:      []string{"198.51.100.1"},
		StaticMappings: map[string]string{"10.0.0.2": "198.51.100.2"},
	}
	MigrateNatFromLegacy(&st, leg)
	if len(st.Nat.IPv4.PolicyRoutes) != 1 || st.Nat.IPv4.PolicyRoutes[0] != "10.1.0.0/16" {
		t.Fatalf("policy routes: %+v", st.Nat.IPv4.PolicyRoutes)
	}
	if st.Nat.IPv4.StaticMappings["10.0.0.2"] != "198.51.100.2" {
		t.Fatalf("static: %+v", st.Nat.IPv4.StaticMappings)
	}
}

func TestLoadNatLegacyJSON(t *testing.T) {
	raw := `{
		"setup_complete": true,
		"policy_routes": ["172.16.0.0/12"],
		"shared_ips": ["203.0.113.5"],
		"shaper": {"policy_cidr": "10.0.0.0/8", "default_profile": {"down":"1mbit","up":"1mbit"}, "profiles":[]},
		"firewall": {},
		"network": {},
		"dhcp": {},
		"vpn": {"wireguards":[{"id":"default","name":"default","mode":"server","interface":"wg0","listen_port":51820,"address":"10.200.0.1/24","peers":[]}]}
	}`
	var disk struct {
		State
	}
	var leg map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &leg); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(raw), &disk.State); err != nil {
		t.Fatal(err)
	}
	var legacy natLegacyFields
	if v, ok := leg["policy_routes"]; ok {
		_ = json.Unmarshal(v, &legacy.PolicyRoutes)
	}
	if v, ok := leg["shared_ips"]; ok {
		_ = json.Unmarshal(v, &legacy.SharedIPs)
	}
	MigrateNatFromLegacy(&disk.State, legacy)
	ensureNatDefaults(&disk.State.Nat)
	if disk.State.Nat.IPv4.PolicyRoutes[0] != "172.16.0.0/12" {
		t.Fatalf("got %+v", disk.State.Nat.IPv4.PolicyRoutes)
	}
}

func TestMigrateNatFromLegacyPartial(t *testing.T) {
	st := State{Nat: NatState{IPv4: NatIPv4State{
		PolicyRoutes: []string{"10.0.0.0/8"},
	}}}
	leg := natLegacyFields{SharedIPs: []string{"198.51.100.1"}}
	MigrateNatFromLegacy(&st, leg)
	if len(st.Nat.IPv4.SharedIPs) != 1 {
		t.Fatalf("shared_ips not migrated: %+v", st.Nat.IPv4.SharedIPs)
	}
}

func TestValidateNptv6Rule(t *testing.T) {
	if err := ValidateNptv6Rule(Nptv6Rule{
		InternalPrefix: "fd00::/48",
		ExternalPrefix: "2001:db8::/48",
	}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateNptv6Rule(Nptv6Rule{
		InternalPrefix: "fd00::/48",
		ExternalPrefix: "2001:db8::/64",
	}); err == nil {
		t.Fatal("expected length mismatch error")
	}
}

func TestEnsureDNS64DefaultsVPNListen(t *testing.T) {
	d := DNS64Config{Mode: DNS64ModeLocal, ServeToClients: false}
	EnsureDNS64Defaults(&d)
	if d.UnboundListen != "" {
		t.Fatalf("vpn mode should keep empty unbound_listen, got %q", d.UnboundListen)
	}
	d2 := DNS64Config{Mode: DNS64ModeLocal, ServeToClients: true}
	EnsureDNS64Defaults(&d2)
	if d2.UnboundListen != "127.0.0.1:5353" {
		t.Fatalf("dhcp relay mode want 127.0.0.1:5353, got %q", d2.UnboundListen)
	}
}
