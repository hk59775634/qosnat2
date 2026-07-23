package store

import (
	"strings"
	"testing"
)

func TestIfaceHostIPv4s(t *testing.T) {
	got := IfaceHostIPv4s([]string{"10.0.0.1/24", "10.0.0.1/32", " 203.0.113.5 ", "bad", "2001:db8::1/64"})
	if len(got) != 2 || got[0] != "10.0.0.1" || got[1] != "203.0.113.5" {
		t.Fatalf("got=%v", got)
	}
}

func TestSyncIfacePolicyRouting(t *testing.T) {
	st := &State{}
	st.Network.WanLinks = []WanLink{
		{ID: "wan-main", Device: "ens18", Gateway: "192.0.2.1", Enabled: true, Metric: 100},
		{ID: "iface-pr-old", Device: "ens19", Gateway: "198.51.100.1", Enabled: true, IfaceManaged: true, PolicyOnly: true},
	}
	st.Network.EgressPolicies = []EgressPolicy{
		{ID: "eg-user", SrcCIDR: "10.0.0.0/24", WanLinkID: "wan-main", Enabled: true, Priority: 100},
		{ID: "auto-iface-pr-old-1-2-3-4", SrcCIDR: "1.2.3.4/32", WanLinkID: "iface-pr-old", Enabled: true, NoSNAT: true},
	}
	up := true
	dhcp := false
	gw := "198.51.100.1"
	pr := true
	UpsertIfaceConfig(st, "ens19", []string{"198.51.100.10/24", "198.51.100.11/32"}, &up, &dhcp, &gw, &pr)

	SyncIfacePolicyRouting(st)

	if len(st.Network.WanLinks) != 2 {
		t.Fatalf("wan links=%d %+v", len(st.Network.WanLinks), st.Network.WanLinks)
	}
	w, ok := FindWanLink(st.Network.WanLinks, IfacePolicyWanLinkID("ens19"))
	if !ok || !w.IfaceManaged || !w.PolicyOnly || w.Gateway != gw || w.Device != "ens19" {
		t.Fatalf("derived wan=%v %+v", ok, w)
	}
	if !IsIfacePolicyWanLink(w) {
		t.Fatal("expected iface policy wan")
	}

	var ifaceEg int
	for _, p := range st.Network.EgressPolicies {
		if IsIfacePolicyEgress(p) {
			ifaceEg++
			if !p.NoSNAT || p.WanLinkID != w.ID || p.Priority != IfacePolicyEgressPrio {
				t.Fatalf("egress %+v", p)
			}
		}
	}
	if ifaceEg != 2 {
		t.Fatalf("iface egress count=%d policies=%+v", ifaceEg, st.Network.EgressPolicies)
	}
	if _, ok := FindEgressPolicy(st.Network.EgressPolicies, "eg-user"); !ok {
		t.Fatal("user egress should remain")
	}

	prOff := false
	UpsertIfaceConfig(st, "ens19", nil, nil, nil, nil, &prOff)
	SyncIfacePolicyRouting(st)
	for _, w := range st.Network.WanLinks {
		if IsIfacePolicyWanLink(w) {
			t.Fatalf("should remove derived wan: %+v", w)
		}
	}
	for _, p := range st.Network.EgressPolicies {
		if IsIfacePolicyEgress(p) {
			t.Fatalf("should remove derived egress: %+v", p)
		}
	}
}

func TestSyncIfaceMainGatewayRoutes(t *testing.T) {
	st := &State{}
	up := true
	dhcp := false
	gwMain := "103.127.237.21"
	prOff := false
	UpsertIfaceConfig(st, "ens18", []string{"103.127.237.22/30"}, &up, &dhcp, &gwMain, &prOff)
	gwPR := "109.244.68.1"
	prOn := true
	UpsertIfaceConfig(st, "ens20", []string{"109.244.68.67/24"}, &up, &dhcp, &gwPR, &prOn)

	SyncIfacePolicyRouting(st)

	var mainDefault, policyTable int
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, ifaceGwRouteCommentPrefix) {
			mainDefault++
			if r.Dest != "default" || r.Gateway != gwMain || r.Device != "ens18" || r.Metric != IfaceMainGatewayMetric {
				t.Fatalf("main gw route: %+v", r)
			}
		}
	}
	if mainDefault != 1 {
		t.Fatalf("main gateway routes=%d %+v", mainDefault, st.Routes)
	}
	for _, r := range st.Routes {
		if r.Table == 201 || strings.Contains(r.Comment, "egress") {
			policyTable++
		}
	}
	_ = policyTable
	SyncWanRoutes(st)
	SyncEgressRoutes(st)
	foundMain := false
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, ifaceGwRouteCommentPrefix) {
			foundMain = true
		}
		// 策略口不得再写一条主表 default
		if r.Dest == "default" && r.Device == "ens20" && r.Table == 0 {
			t.Fatalf("ens20 must not own main default: %+v", r)
		}
	}
	if !foundMain {
		t.Fatal("iface main gateway should survive SyncWanRoutes/SyncEgressRoutes")
	}
}

func TestValidateIfacePolicyRouting(t *testing.T) {
	if err := ValidateIfacePolicyRouting(IfaceConfig{PolicyRouting: true, Gateway: "1.2.3.4", IPv4: []string{"1.2.3.5/24"}}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateIfacePolicyRouting(IfaceConfig{PolicyRouting: true, Gateway: "", IPv4: []string{"1.2.3.5/24"}}); err == nil {
		t.Fatal("expected gateway required")
	}
	if err := ValidateIfacePolicyRouting(IfaceConfig{PolicyRouting: true, Gateway: "1.2.3.4", DHCP4: true, IPv4: []string{"1.2.3.5/24"}}); err == nil {
		t.Fatal("expected dhcp conflict")
	}
	if err := ValidateIfacePolicyRouting(IfaceConfig{PolicyRouting: true, Gateway: "1.2.3.4"}); err == nil {
		t.Fatal("expected ipv4 required")
	}
}

func FindEgressPolicy(list []EgressPolicy, id string) (EgressPolicy, bool) {
	for _, p := range list {
		if p.ID == id {
			return p, true
		}
	}
	return EgressPolicy{}, false
}
