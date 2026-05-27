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
}
