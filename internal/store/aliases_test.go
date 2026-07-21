package store

import "testing"

func TestAliasReferencedByRules(t *testing.T) {
	rules := []FilterRule{{ID: "fr-1", SrcAlias: "servers", Chain: "forward", Action: "accept"}}
	if !AliasReferencedByRules(rules, "servers") {
		t.Fatal("expected reference")
	}
	if AliasReferencedByRules(rules, "other") {
		t.Fatal("unexpected reference")
	}
}

func TestValidateFilterRuleAliases(t *testing.T) {
	aliases := []AliasSet{{Name: "servers", Type: "ipv4_addr", Members: []string{"10.0.0.0/24"}}}
	r := FilterRule{Chain: "forward", Action: "accept", SrcAlias: "missing"}
	if err := ValidateFilterRuleAliases(r, aliases); err == nil {
		t.Fatal("expected missing alias error")
	}
	r.SrcAlias = "servers"
	if err := ValidateFilterRuleAliases(r, aliases); err != nil {
		t.Fatal(err)
	}
	r.SrcAlias = "ports"
	aliases = append(aliases, AliasSet{Name: "ports", Type: "port", Members: []string{"80", "443"}})
	if err := ValidateFilterRuleAliases(r, aliases); err == nil {
		t.Fatal("expected port alias rejection on src_alias")
	}
}

func TestNormalizeAliasASNOK(t *testing.T) {
	a := &AliasSet{Name: "as13335", Type: "asn", ASN: 13335, Members: []string{"203.0.113.0/24"}}
	if err := NormalizeAlias(a); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeAliasPort(t *testing.T) {
	a := &AliasSet{Name: "web", Type: "port", Members: []string{"80", "443", "8000-8010"}}
	if err := NormalizeAlias(a); err != nil {
		t.Fatal(err)
	}
	if len(a.Members) != 3 {
		t.Fatalf("members=%v", a.Members)
	}
}

func TestNormalizeAliasMemberInvalid(t *testing.T) {
	a := &AliasSet{Name: "bad", Type: "ipv4_addr", Members: []string{"not-a-cidr"}}
	if err := NormalizeAlias(a); err == nil {
		t.Fatal("expected invalid member error")
	}
}

func TestNormalizeFilterRulePort(t *testing.T) {
	r := FilterRule{Chain: "forward", Action: "drop", DstPort: 70000}
	if err := NormalizeFilterRule(&r); err == nil {
		t.Fatal("expected port validation error")
	}
}

func TestNormalizeFilterRuleIPv6(t *testing.T) {
	r := FilterRule{
		Chain: "forward", Action: "accept", IPVersion: "ipv6",
		SrcAddr: "2001:db8::1", Enabled: true,
	}
	if err := NormalizeFilterRule(&r); err != nil {
		t.Fatal(err)
	}
	if r.IPVersion != "ipv6" {
		t.Fatalf("ip_version: %q", r.IPVersion)
	}
}

func TestNormalizeFilterRuleIPv6RejectIPv4Addr(t *testing.T) {
	r := FilterRule{
		Chain: "forward", Action: "accept", IPVersion: "ipv6",
		SrcAddr: "10.0.0.1",
	}
	if err := NormalizeFilterRule(&r); err == nil {
		t.Fatal("expected ipv6 validation error for ipv4 addr")
	}
}
