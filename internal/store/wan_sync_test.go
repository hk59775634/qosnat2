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
		t.Fatalf("routes len=%d want 2", len(st.Routes))
	}
	var nh *RouteEntry
	for i := range st.Routes {
		if st.Routes[i].ID == "wan-nh-100" {
			nh = &st.Routes[i]
		}
	}
	if nh == nil || len(nh.Nexthops) != 2 {
		t.Fatalf("expected nexthop route, got %+v", st.Routes)
	}
	if nh.Nexthops[0].Weight != 3 || nh.Nexthops[1].Weight != 1 {
		t.Fatalf("weights %+v", nh.Nexthops)
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
