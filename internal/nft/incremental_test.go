package nft

import "testing"

func TestFindRuleHandle(t *testing.T) {
	listing := `		iifname "eth0" accept comment "qosnat2:rid:fr-abc" # handle 10
		drop comment "other" # handle 11`
	got := findRuleHandle(listing, "qosnat2:rid:fr-abc")
	if got != "10" {
		t.Fatalf("got %q want 10", got)
	}
	if findRuleHandle(listing, "missing") != "" {
		t.Fatal("expected empty for missing marker")
	}
}

func TestIncrementalEnabled(t *testing.T) {
	t.Setenv("QOSNAT_NFT_INCREMENTAL", "1")
	if !IncrementalEnabled() {
		t.Fatal("expected enabled")
	}
	t.Setenv("QOSNAT_NFT_INCREMENTAL", "0")
	if IncrementalEnabled() {
		t.Fatal("expected disabled")
	}
}
