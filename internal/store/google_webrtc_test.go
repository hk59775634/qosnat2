package store

import "testing"

func TestGoogleWebRTCBlockAliases(t *testing.T) {
	aliases := GoogleWebRTCBlockAliases()
	if len(aliases) != 3 {
		t.Fatalf("aliases len=%d", len(aliases))
	}
	for i := range aliases {
		a := aliases[i]
		if err := NormalizeAlias(&a); err != nil {
			t.Fatalf("NormalizeAlias %s: %v", a.Name, err)
		}
		aliases[i] = a
	}
	if aliases[0].Name != AliasGoogleWebRTCStun || aliases[0].Type != "fqdn" {
		t.Fatalf("stun alias: %+v", aliases[0])
	}
	if len(aliases[0].Domains) < 5 {
		t.Fatalf("stun domains=%v", aliases[0].Domains)
	}
	if aliases[1].Name != AliasGoogleWebRTCMedia || len(aliases[1].Members) < 3 {
		t.Fatalf("media alias: %+v", aliases[1])
	}
	if aliases[2].Name != AliasGoogleWebRTCPorts || aliases[2].Type != "port" {
		t.Fatalf("ports alias: %+v", aliases[2])
	}
}

func TestGoogleWebRTCBlockRules(t *testing.T) {
	aliases := GoogleWebRTCBlockAliases()
	for i := range aliases {
		if err := NormalizeAlias(&aliases[i]); err != nil {
			t.Fatal(err)
		}
	}
	rules := GoogleWebRTCBlockRules()
	if len(rules) != 2 {
		t.Fatalf("rules len=%d", len(rules))
	}
	for i := range rules {
		r := rules[i]
		if err := NormalizeFilterRule(&r); err != nil {
			t.Fatalf("NormalizeFilterRule %s: %v", r.ID, err)
		}
		if err := ValidateFilterRuleAliases(r, aliases); err != nil {
			t.Fatalf("ValidateFilterRuleAliases %s: %v", r.ID, err)
		}
		if err := ValidateFilterRulePortAliases(r, aliases); err != nil {
			t.Fatalf("ValidateFilterRulePortAliases %s: %v", r.ID, err)
		}
		line := r.NftRuleLine()
		if line == "" {
			t.Fatalf("empty nft line for %s", r.ID)
		}
	}
}

func TestUpsertAliasesAndRulesIdempotent(t *testing.T) {
	a1, added, updated := UpsertAliasesByName(nil, GoogleWebRTCBlockAliases())
	if added != 3 || updated != 0 {
		t.Fatalf("first aliases added=%d updated=%d", added, updated)
	}
	a2, added, updated := UpsertAliasesByName(a1, GoogleWebRTCBlockAliases())
	if added != 0 || updated != 3 || len(a2) != 3 {
		t.Fatalf("second aliases added=%d updated=%d len=%d", added, updated, len(a2))
	}

	r1, added, updated := UpsertFilterRulesByID(nil, GoogleWebRTCBlockRules())
	if added != 2 || updated != 0 {
		t.Fatalf("first rules added=%d updated=%d", added, updated)
	}
	r2, added, updated := UpsertFilterRulesByID(r1, GoogleWebRTCBlockRules())
	if added != 0 || updated != 2 || len(r2) != 2 {
		t.Fatalf("second rules added=%d updated=%d len=%d", added, updated, len(r2))
	}
}
