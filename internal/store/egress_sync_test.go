package store

import "testing"

func TestPrimaryWanLinkID(t *testing.T) {
	links := []WanLink{
		{ID: "wan-b", Device: "eth1", Metric: 110, Enabled: true},
		{ID: "wan-a", Device: "eth0", Metric: 100, Enabled: true},
	}
	if got := PrimaryWanLinkID(links); got != "wan-a" {
		t.Fatalf("primary=%q want wan-a", got)
	}
}

func TestWanLinkUsesPolicyTableOnly(t *testing.T) {
	links := []WanLink{
		{ID: "wan-a", Device: "eth0", Metric: 100, Enabled: true},
		{ID: "wan-b", Device: "eth1", Metric: 110, Enabled: true},
	}
	if WanLinkUsesPolicyTableOnly(links[0], links) {
		t.Fatal("primary should use main table")
	}
	if !WanLinkUsesPolicyTableOnly(links[1], links) {
		t.Fatal("secondary should use policy table only")
	}
}

func TestSyncWanRoutesMultiWanPrimaryOnly(t *testing.T) {
	st := &State{
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "wan-a", Gateway: "1.1.1.1", Device: "eth0", Metric: 100, Enabled: true},
				{ID: "wan-b", Gateway: "2.2.2.2", Device: "eth1", Metric: 110, Enabled: true},
			},
		},
	}
	SyncWanRoutes(st)
	if len(st.Routes) != 1 {
		t.Fatalf("routes len=%d want 1 primary in main", len(st.Routes))
	}
	if st.Routes[0].Device != "eth0" {
		t.Fatalf("expected primary eth0, got %+v", st.Routes[0])
	}
}

func TestSyncEgressRoutesPolicyWan(t *testing.T) {
	st := &State{
		Routes: []RouteEntry{{ID: "manual", Dest: "192.168.0.0/24", Gateway: "10.0.0.1", Enabled: true}},
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "wan-a", Device: "eth0", Gateway: "1.1.1.1", Metric: 100, Enabled: true},
				{ID: "wan-us", Name: "US", Device: "ens19", Gateway: "100.64.0.1", Metric: 110, Enabled: true},
			},
			EgressPolicies: []EgressPolicy{
				{ID: "eg-1", CIDR: "10.250.0.0/24", WanLinkID: "wan-us", Enabled: true},
			},
		},
	}
	SyncEgressRoutes(st)
	if len(st.Routes) != 2 {
		t.Fatalf("routes len=%d want manual + policy wan", len(st.Routes))
	}
	var us *RouteEntry
	for i := range st.Routes {
		if st.Routes[i].Device == "ens19" {
			us = &st.Routes[i]
		}
	}
	if us == nil {
		t.Fatal("missing ens19 policy route")
	}
	if us.Table != 202 || us.Gateway != "100.64.0.1" {
		t.Fatalf("bad egress route: %+v", us)
	}
	if !us.Locked || us.Source != RouteSourceEgress {
		t.Fatalf("expected locked egress route, got %+v", us)
	}
}

func TestSyncEgressRoutesSecondaryPolicyTable(t *testing.T) {
	st := &State{
		Network: NetworkState{
			WanLinks: []WanLink{
				{ID: "wan-a", Device: "eth0", Gateway: "1.1.1.1", Metric: 100, Enabled: true},
				{ID: "wan-b", Device: "eth1", Gateway: "2.2.2.2", Metric: 110, Enabled: true},
			},
		},
	}
	SyncEgressRoutes(st)
	if len(st.Routes) != 1 {
		t.Fatalf("routes len=%d want secondary policy table only", len(st.Routes))
	}
	if st.Routes[0].Table != 202 || st.Routes[0].Device != "eth1" {
		t.Fatalf("bad policy route: %+v", st.Routes[0])
	}
}

func TestSyncEgressRoutesSingleWan(t *testing.T) {
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
		if st.Routes[i].Comment == egressRouteCommentPrefix+"wan-us" {
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

func TestEnrichRouteEntryLegacyEgress(t *testing.T) {
	st := &State{
		Routes: []RouteEntry{{
			ID: "eg-1", Dest: "default", Gateway: "100.64.0.1", Device: "ens19", Table: 201,
			Comment: egressRouteCommentPrefix + "eg-1",
		}},
		Network: NetworkState{
			WanLinks:       []WanLink{{ID: "wan-us", Name: "US", Device: "ens19", Gateway: "100.64.0.1", Enabled: true}},
			EgressPolicies: []EgressPolicy{{ID: "eg-1", CIDR: "10.250.0.0/24", WanLinkID: "wan-us", Enabled: true}},
		},
	}
	r := EnrichRouteEntry(st.Routes[0], *st)
	if !r.Locked || r.Source != RouteSourceEgress || r.SourceNote == "" {
		t.Fatalf("enrich: %+v", r)
	}
}

func TestFilterPolicyRoutesForWAN(t *testing.T) {
	in := []string{"10.0.0.0/8", "10.250.0.0/24"}
	out := FilterPolicyRoutesForWAN(in, []string{"10.250.0.0/24"})
	if len(out) != 1 || out[0] != "10.0.0.0/8" {
		t.Fatalf("got %v", out)
	}
}
