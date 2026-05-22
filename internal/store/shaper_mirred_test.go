package store

import "testing"

func TestCollectMirredCIDRs(t *testing.T) {
	sh := ShaperState{
		PolicyCIDR: "10.0.0.0/8",
		Profiles: []ProfileEntry{
			{CIDR: "10.0.0.0/8"},
			{CIDR: "100.64.0.0/24"},
			{CIDR: "10.0.0.1/32"},
		},
	}
	got := CollectMirredCIDRs(sh)
	if len(got) != 3 {
		t.Fatalf("want 3 cidrs, got %v", got)
	}
	m := map[string]bool{}
	for _, c := range got {
		m[c] = true
	}
	for _, want := range []string{"10.0.0.0/8", "100.64.0.0/24", "10.0.0.1/32"} {
		if !m[want] {
			t.Fatalf("missing %s in %v", want, got)
		}
	}
}
