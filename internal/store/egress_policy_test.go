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

func TestNormalizeEgressPolicy(t *testing.T) {
	p := &EgressPolicy{CIDR: "10.250.0.0/24", WanLinkID: "wan-1"}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.Priority != 100 || p.ID == "" {
		t.Fatalf("got %+v", p)
	}
	if p.Match != "source" {
		t.Fatalf("match=%q want source", p.Match)
	}
}

func TestNormalizeEgressPolicyMatch(t *testing.T) {
	p := &EgressPolicy{CIDR: "173.245.48.0/20", WanLinkID: "wan-1", Match: "destination"}
	if err := NormalizeEgressPolicy(p); err != nil {
		t.Fatal(err)
	}
	if p.Match != "destination" {
		t.Fatalf("match=%q", p.Match)
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
