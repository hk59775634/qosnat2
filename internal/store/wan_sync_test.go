package store

import "testing"

func TestSyncWanRoutesNexthopWeight(t *testing.T) {
	st := &State{
		Routes: []RouteEntry{{ID: "manual", Dest: "192.168.1.0/24", Gateway: "10.0.0.1", Enabled: true}},
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "w1", Gateway: "1.1.1.1", Device: "eth0", Metric: 100, Weight: 3, Enabled: true},
				{ID: "w2", Gateway: "2.2.2.2", Device: "eth1", Metric: 100, Weight: 1, Enabled: true},
			},
		},
	}
	SyncWanRoutes(st)
	if len(st.Routes) != 2 {
		t.Fatalf("routes len=%d want manual + primary default", len(st.Routes))
	}
	var primary *RouteEntry
	for i := range st.Routes {
		if st.Routes[i].Comment == wanRouteCommentPrefix+"w1" {
			primary = &st.Routes[i]
		}
	}
	if primary == nil || primary.Device != "eth0" {
		t.Fatalf("expected primary w1 in main, got %+v", st.Routes)
	}
}

func TestSyncWanRoutesSkipPolicyOnly(t *testing.T) {
	st := &State{
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "warp", Device: "wgcf", Metric: 90, Enabled: true, PolicyOnly: true},
			},
		},
	}
	SyncWanRoutes(st)
	if len(st.Routes) != 0 {
		t.Fatalf("policy_only wan should not create main default route: %+v", st.Routes)
	}
}
