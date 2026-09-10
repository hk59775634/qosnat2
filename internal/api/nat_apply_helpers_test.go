package api

import "testing"

import "github.com/hk59775634/qosnat2/internal/store"

func TestNatIPv4ConntrackCIDRsDiff(t *testing.T) {
	oldN := store.NatIPv4State{
		PolicyRoutes:   []string{"10.0.0.0/8", "198.18.250.0/24"},
		StaticMappings: map[string]string{"10.1.0.1": "203.0.113.1"},
	}
	newN := store.NatIPv4State{
		PolicyRoutes:   []string{"10.0.0.0/8", "198.19.0.0/24"},
		StaticMappings: map[string]string{"10.1.0.1": "203.0.113.1", "10.1.0.2": "203.0.113.2"},
	}
	got := natIPv4ConntrackCIDRs(oldN, newN)
	want := map[string]bool{"198.18.250.0/24": false, "198.19.0.0/24": false, "10.1.0.2": false}
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
