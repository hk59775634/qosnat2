package policyroute

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestPlanDeltaAddRemoveAndNameOnly(t *testing.T) {
	a := store.EgressPolicy{
		ID: "eg-1", Name: "a", SrcCIDR: "10.0.0.0/8", WanLinkID: "wan-1",
		Priority: 100, Enabled: true, SNATIP: "203.0.113.1",
	}
	d := PlanDelta(nil, []store.EgressPolicy{a})
	if !d.IPRules || !d.NFT {
		t.Fatalf("add: %+v", d)
	}
	d = PlanDelta([]store.EgressPolicy{a}, nil)
	if !d.IPRules || !d.NFT {
		t.Fatalf("remove: %+v", d)
	}
	b := a
	b.Name = "renamed"
	d = PlanDelta([]store.EgressPolicy{a}, []store.EgressPolicy{b})
	if d.IPRules || d.NFT {
		t.Fatalf("name-only should skip dataplane: %+v", d)
	}
}

func TestPlanDeltaSNATAndCIDR(t *testing.T) {
	a := store.EgressPolicy{
		ID: "eg-1", SrcCIDR: "10.0.0.0/8", WanLinkID: "wan-1",
		Priority: 100, Enabled: true, SNATIP: "203.0.113.1",
	}
	snat := a
	snat.SNATIP = "203.0.113.2"
	d := PlanDelta([]store.EgressPolicy{a}, []store.EgressPolicy{snat})
	if d.IPRules {
		t.Fatal("SNAT-only should not rewrite ip rules")
	}
	if !d.NFT {
		t.Fatal("SNAT-only should update nft")
	}
	cidr := a
	cidr.SrcCIDR = "10.8.0.0/24"
	d = PlanDelta([]store.EgressPolicy{a}, []store.EgressPolicy{cidr})
	if !d.IPRules || !d.NFT {
		t.Fatalf("cidr change: %+v", d)
	}
}

func TestReferencedWansChanged(t *testing.T) {
	a := store.EgressPolicy{ID: "eg-1", WanLinkID: "wan-1", Enabled: true}
	b := store.EgressPolicy{ID: "eg-1", WanLinkID: "wan-2", Enabled: true}
	if !ReferencedWansChanged([]store.EgressPolicy{a}, []store.EgressPolicy{b}) {
		t.Fatal("wan change")
	}
	if ReferencedWansChanged([]store.EgressPolicy{a}, []store.EgressPolicy{a}) {
		t.Fatal("same wan")
	}
}

func TestChangedCIDRsIncludesOldAndNew(t *testing.T) {
	st := store.State{}
	prev := []store.EgressPolicy{{ID: "eg-1", SrcCIDR: "10.0.0.0/8", Enabled: true, WanLinkID: "w"}}
	next := []store.EgressPolicy{{ID: "eg-1", SrcCIDR: "10.8.0.0/24", Enabled: true, WanLinkID: "w"}}
	got := ChangedCIDRs(st, prev, next)
	want := map[string]bool{"10.0.0.0/8": false, "10.8.0.0/24": false}
	for _, c := range got {
		if _, ok := want[c]; ok {
			want[c] = true
		}
	}
	for c, ok := range want {
		if !ok {
			t.Fatalf("missing %s in %v", c, got)
		}
	}
}
