package store

import "testing"

func TestWanLinkRouteTableStable(t *testing.T) {
	links := []WanLink{
		{ID: "wan-b", Enabled: true},
		{ID: "wan-a", Enabled: true},
	}
	if got := WanLinkRouteTable("wan-a", links); got != 201 {
		t.Fatalf("wan-a table=%d want 201", got)
	}
	if got := WanLinkRouteTable("wan-b", links); got != 202 {
		t.Fatalf("wan-b table=%d want 202", got)
	}
}

func TestWanLinkRouteTableSkipsDisabled(t *testing.T) {
	links := []WanLink{
		{ID: "wan-off", Enabled: false},
		{ID: "wan-a", Enabled: true},
	}
	if got := WanLinkRouteTable("wan-off", links); got != 0 {
		t.Fatalf("disabled link table=%d want 0", got)
	}
	if got := WanLinkRouteTable("wan-a", links); got != 201 {
		t.Fatalf("wan-a table=%d want 201 (disabled must not shift index)", got)
	}
}

func TestNormalizeEgressPolicy(t *testing.T) {
	p := &EgressPolicy{CIDR: "10.250.0.0/24", WanLinkID: "wan-1"}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.Priority != 100 || p.ID == "" {
		t.Fatalf("got %+v", p)
	}
	if p.SrcCIDR != "10.250.0.0/24" {
		t.Fatalf("src_cidr=%q want migrated from cidr", p.SrcCIDR)
	}
}

func TestNormalizeEgressPolicyMatch(t *testing.T) {
	p := &EgressPolicy{CIDR: "173.245.48.0/20", WanLinkID: "wan-1", Match: "destination"}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.DstCIDR != "173.245.48.0/20" {
		t.Fatalf("dst_cidr=%q", p.DstCIDR)
	}
}

func TestNormalizeEgressPolicyBothEndpoints(t *testing.T) {
	p := &EgressPolicy{
		SrcCIDR: "10.250.0.0/24", DstAlias: "google_ipv4",
		WanLinkID: "wan-1",
	}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeEgressPolicyHostIPv4(t *testing.T) {
	p := &EgressPolicy{SrcCIDR: "10.0.0.42", WanLinkID: "wan-1"}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.SrcCIDR != "10.0.0.42/32" {
		t.Fatalf("src_cidr=%q want /32", p.SrcCIDR)
	}
}

func TestNormalizeEgressPolicySrcIfaceOnly(t *testing.T) {
	p := &EgressPolicy{SrcIface: "wg0", WanLinkID: "wan-1"}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
}

func TestExpandEgressIPRules_iif(t *testing.T) {
	p := EgressPolicy{
		SrcIface: "wg0", SrcCIDR: "10.8.0.0/24",
		Priority: 100, Enabled: true,
	}
	rules, err := ExpandEgressIPRules(p, 201, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].Iif != "wg0" || rules[0].From != "10.8.0.0/24" || rules[0].Mode != "source" {
		t.Fatalf("got %+v", rules)
	}
}

func TestEgressSNATMatchClause_iif(t *testing.T) {
	cl := EgressSNATMatchClause(EgressPolicy{SrcIface: "vpns0", SrcCIDR: "10.250.0.5/32"})
	want := `iifname "vpns0" ip saddr 10.250.0.5/32`
	if cl != want {
		t.Fatalf("got %q want %q", cl, want)
	}
}

func TestExpandEgressIPRules_both(t *testing.T) {
	p := EgressPolicy{
		SrcCIDR: "10.0.0.0/8", DstCIDR: "8.8.8.0/24",
		Priority: 100, Enabled: true,
	}
	rules, err := ExpandEgressIPRules(p, 201, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 1 || rules[0].From != "10.0.0.0/8" || rules[0].To != "8.8.8.0/24" || rules[0].Mode != "both" {
		t.Fatalf("got %+v", rules)
	}
}

func TestEgressPolicySourceMatchCIDRs_skipsDestination(t *testing.T) {
	policies := []EgressPolicy{
		{ID: "eg-src", CIDR: "10.250.0.0/24", Match: "source", Enabled: true},
		{ID: "eg-dst", CIDR: "173.245.48.0/20", Match: "destination", Enabled: true},
	}
	got := EgressPolicySourceMatchCIDRs(policies)
	if len(got) != 1 || got[0] != "10.250.0.0/24" {
		t.Fatalf("got %v", got)
	}
	all := EgressPolicyCIDRs(policies)
	if len(all) != 1 || all[0] != "10.250.0.0/24" {
		t.Fatalf("EgressPolicyCIDRs got %v", all)
	}
}

func TestEgressSNATAddrPrefix(t *testing.T) {
	if EgressSNATAddrPrefix("destination") != "ip daddr" {
		t.Fatal("destination")
	}
	if EgressSNATAddrPrefix("source") != "ip saddr" {
		t.Fatal("source")
	}
}

func TestCloudflareCDNCIDRsV4(t *testing.T) {
	list := CloudflareCDNCIDRsV4()
	if len(list) < 10 {
		t.Fatalf("unexpected list size: %d", len(list))
	}
	if list[0] == "" {
		t.Fatal("first cidr empty")
	}
}

func TestResolveEgressPolicies_WarpMasqueradeFallback(t *testing.T) {
	st := State{
		Network: NetworkState{
			WanLinks: []WanLink{
				{
					ID:          WanLinkIDWarp,
					Device:      "CloudflareWARP",
					Enabled:     true,
					PolicyOnly:  true,
					WarpManaged: true,
				},
			},
			EgressPolicies: []EgressPolicy{
				{ID: "eg-1", CIDR: "10.88.0.0/24", WanLinkID: WanLinkIDWarp, Enabled: true, Priority: 100},
			},
		},
	}
	resolved := ResolveEgressPolicies(st, func(device string) (string, error) {
		return "", nil
	})
	if len(resolved) != 1 {
		t.Fatalf("resolved len=%d want 1", len(resolved))
	}
	if !resolved[0].Masquerade {
		t.Fatal("warp egress should fallback to masquerade when no snat ip")
	}
	if resolved[0].SNATIP != "" {
		t.Fatalf("unexpected snat ip: %q", resolved[0].SNATIP)
	}
}
