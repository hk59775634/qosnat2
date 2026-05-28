package policyroute

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRuleDirections_source(t *testing.T) {
	mainDir, policyDir := ruleDirections("source")
	if mainDir != "to" || policyDir != "from" {
		t.Fatalf("source: main=%q policy=%q", mainDir, policyDir)
	}
}

func TestRuleDirections_destination(t *testing.T) {
	mainDir, policyDir := ruleDirections("destination")
	if mainDir != "from" || policyDir != "to" {
		t.Fatalf("destination: main=%q policy=%q", mainDir, policyDir)
	}
}

func TestRuleDirections_default(t *testing.T) {
	mainDir, policyDir := ruleDirections("")
	if mainDir != "to" || policyDir != "from" {
		t.Fatalf("empty match: main=%q policy=%q", mainDir, policyDir)
	}
}

func TestDelRulesSymmetricWithAddRules(t *testing.T) {
	for _, match := range []string{"source", "destination", ""} {
		mainAdd, policyAdd := ruleDirections(match)
		mainDel, policyDel := ruleDirections(match)
		if mainAdd != mainDel || policyAdd != policyDel {
			t.Fatalf("match %q: add (%s,%s) del (%s,%s)", match, mainAdd, policyAdd, mainDel, policyDel)
		}
	}
}

func TestCheckUnresolvedEgress_noSNAT(t *testing.T) {
	st := store.State{
		Network: store.NetworkState{
			WanLinks: []store.WanLink{
				{ID: "wan-1", Device: "eth1", Gateway: "10.0.0.1", Enabled: true},
			},
			EgressPolicies: []store.EgressPolicy{
				{ID: "eg-1", CIDR: "10.250.0.0/24", WanLinkID: "wan-1", Enabled: true, Priority: 100},
			},
		},
	}
	err := checkUnresolvedEgress(st, nil)
	if err == nil {
		t.Fatal("expected error when no SNAT and no resolver in resolved list")
	}
}

func TestCheckUnresolvedEgress_warpNotReady(t *testing.T) {
	st := store.State{
		Network: store.NetworkState{
			WanLinks: []store.WanLink{
				{
					ID:          store.WanLinkIDWarp,
					Device:      "CloudflareWARP",
					Enabled:     true,
					PolicyOnly:  true,
					WarpManaged: true,
				},
			},
			EgressPolicies: []store.EgressPolicy{
				{ID: "eg-1", CIDR: "10.88.0.0/24", WanLinkID: store.WanLinkIDWarp, Enabled: true, Priority: 100},
			},
		},
	}
	err := checkUnresolvedEgress(st, nil)
	if err == nil {
		t.Fatal("expected warp unresolved error")
	}
}
