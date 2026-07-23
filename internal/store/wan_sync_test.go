package store

import (
	"strings"
	"testing"
)

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

func TestSyncWanRoutesSkipWhenIfaceGatewayCoversMain(t *testing.T) {
	st := &State{
		Routes: []RouteEntry{{
			ID: "iface-gw-ens18", Dest: "default", Gateway: "103.127.237.21", Device: "ens18",
			Metric: 100, Comment: ifaceGwRouteCommentPrefix + "ens18", Enabled: true,
		}},
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "wan-wg-a", Gateway: "100.64.1.1", Device: "wg0", Metric: 100, Enabled: true},
				{ID: "wan-wg-b", Gateway: "100.64.1.2", Device: "wg0", Metric: 200, Enabled: true},
			},
		},
	}
	SyncWanRoutes(st)
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, wanRouteCommentPrefix) {
			t.Fatalf("wg0 wan should not create main default when iface-gw exists: %+v", r)
		}
	}
}
