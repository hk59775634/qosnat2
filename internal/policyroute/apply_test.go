package policyroute

import (
	"strings"
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

func TestMainBypassSelector_bothUsesReturnPath(t *testing.T) {
	r := store.EgressIPRule{
		From: "10.0.0.0/8",
		To:   "8.8.8.0/24",
		Iif:  "wg0",
		Mode: "both",
	}
	from, to, iif, ok := mainBypassSelector(r)
	if !ok {
		t.Fatal("expected bypass")
	}
	if from != "8.8.8.0/24" || to != "10.0.0.0/8" {
		t.Fatalf("both bypass want return path from=dst to=src, got from=%q to=%q", from, to)
	}
	if iif != "" {
		t.Fatalf("both bypass must omit iif, got %q", iif)
	}
}

func TestMainBypassSelector_sourceAndDestination(t *testing.T) {
	from, to, iif, ok := mainBypassSelector(store.EgressIPRule{From: "10.0.0.0/8", Iif: "lan0", Mode: "source"})
	if !ok || from != "" || to != "10.0.0.0/8" || iif != "lan0" {
		t.Fatalf("source bypass: from=%q to=%q iif=%q ok=%v", from, to, iif, ok)
	}
	from, to, iif, ok = mainBypassSelector(store.EgressIPRule{To: "8.8.8.0/24", Mode: "destination"})
	if !ok || from != "8.8.8.0/24" || to != "" || iif != "" {
		t.Fatalf("destination bypass: from=%q to=%q iif=%q ok=%v", from, to, iif, ok)
	}
}

func TestCheckUnresolvedEgress_missingSNATIP(t *testing.T) {
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

func TestCheckUnresolvedEgress_routeOnlyMissingGateway(t *testing.T) {
	st := store.State{
		Network: store.NetworkState{
			WanLinks: []store.WanLink{
				{ID: "wan-1", Device: "eth1", Gateway: "", Enabled: true},
			},
			EgressPolicies: []store.EgressPolicy{
				{ID: "eg-1", SrcCIDR: "10.250.0.0/24", WanLinkID: "wan-1", NoSNAT: true, Enabled: true, Priority: 100},
			},
		},
	}
	err := checkUnresolvedEgress(st, nil)
	if err == nil || !strings.Contains(err.Error(), "no_snat requires gateway") {
		t.Fatalf("expected no_snat gateway error, got %v", err)
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
