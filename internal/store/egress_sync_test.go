package store

import "testing"

func TestSyncEgressRoutes(t *testing.T) {
	st := &State{
		Routes: []RouteEntry{{ID: "manual", Dest: "192.168.0.0/24", Gateway: "10.0.0.1", Enabled: true}},
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "wan-us", Name: "US", Device: "ens19", Gateway: "100.64.0.1", Enabled: true},
			},
			EgressPolicies: []EgressPolicy{
				{ID: "eg-1", CIDR: "10.250.0.0/24", WanLinkID: "wan-us", Enabled: true},
			},
		},
	}
	SyncEgressRoutes(st)
	if len(st.Routes) != 2 {
		t.Fatalf("routes len=%d want 2", len(st.Routes))
	}
	var eg *RouteEntry
	for i := range st.Routes {
		if st.Routes[i].Comment == egressRouteCommentPrefix+"eg-1" {
			eg = &st.Routes[i]
		}
	}
	if eg == nil {
		t.Fatal("missing egress route")
	}
	if eg.Table != 201 || eg.Device != "ens19" || eg.Gateway != "100.64.0.1" {
		t.Fatalf("bad egress route: %+v", eg)
	}
}

func TestFilterPolicyRoutesForWAN(t *testing.T) {
	in := []string{"10.0.0.0/8", "10.250.0.0/24"}
	out := FilterPolicyRoutesForWAN(in, []string{"10.250.0.0/24"})
	if len(out) != 1 || out[0] != "10.0.0.0/8" {
		t.Fatalf("got %v", out)
	}
}
