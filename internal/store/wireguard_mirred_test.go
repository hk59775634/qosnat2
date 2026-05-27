package store

import "testing"

func TestWireGuardMirredSrcCIDRs(t *testing.T) {
	w := WireGuardState{Enabled: true, Address: "10.200.0.1/24"}
	got := WireGuardMirredSrcCIDRs(w)
	if len(got) != 1 || got[0] != "10.200.0.0/24" {
		t.Fatalf("got %v", got)
	}
	w2 := WireGuardState{Enabled: true, Address: "10.99.1.5"}
	got2 := WireGuardMirredSrcCIDRs(w2)
	if len(got2) != 1 || got2[0] != "10.99.1.5/32" {
		t.Fatalf("got %v", got2)
	}
}
