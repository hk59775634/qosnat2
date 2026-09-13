package store

import "testing"

func TestCollectVXLANAutoEndpointsSkipsDown(t *testing.T) {
	eps := CollectVXLANAutoEndpoints([]VXLANTunnel{
		{ID: "vxlan-a", VNI: 100, Name: "vxlan100", Remote: "198.51.100.20", Port: 4789, Up: true},
		{ID: "vxlan-b", VNI: 200, Remote: "203.0.113.10", Up: false},
	})
	if len(eps) != 1 || eps[0].ID != "vxlan-a" || eps[0].Port != 4789 {
		t.Fatalf("got %+v", eps)
	}
}

func TestVXLANRemoteMatch(t *testing.T) {
	src, ver := VXLANRemoteMatch("198.51.100.20")
	if src != "198.51.100.20/32" || ver != "" {
		t.Fatalf("ipv4: %s %s", src, ver)
	}
	src, ver = VXLANRemoteMatch("2001:db8::1")
	if src != "2001:db8::1/128" || ver != "ipv6" {
		t.Fatalf("ipv6: %s %s", src, ver)
	}
}

func TestBuildAutoInputRulesVXLAN(t *testing.T) {
	vpn := AutoInputVPN{VXLAN: []VXLANAutoEndpoint{
		{ID: "vxlan-a", Iface: "vxlan100", Port: 4789, Remote: "198.51.100.20"},
	}}
	rules := BuildAutoInputRules([]string{"ens18"}, "9443", vpn)
	var found bool
	for _, r := range rules {
		if r.ID == "auto-input-vxlan-vxlan-a-ens18" {
			found = true
			if r.Proto != "udp" || r.DstPort != 4789 || r.SrcAddr != "198.51.100.20/32" || r.Iif != "ens18" {
				t.Fatalf("unexpected rule: %+v", r)
			}
			if !r.System {
				t.Fatal("expected system rule")
			}
		}
	}
	if !found {
		t.Fatalf("missing vxlan input rule in %+v", rules)
	}
}

func TestBuildAutoInputRulesVXLANUnderlayOnly(t *testing.T) {
	vpn := AutoInputVPN{VXLAN: []VXLANAutoEndpoint{
		{ID: "vxlan-a", Iface: "vxlan100", Port: 4789, Remote: "198.51.100.20", Underlay: "ens20"},
	}}
	rules := BuildAutoInputRules([]string{"ens18", "ens20"}, "9443", vpn)
	var ens18, ens20 bool
	for _, r := range rules {
		if r.ID == "auto-input-vxlan-vxlan-a-ens18" {
			ens18 = true
		}
		if r.ID == "auto-input-vxlan-vxlan-a-ens20" {
			ens20 = true
		}
	}
	if ens18 {
		t.Fatal("underlay ens20 must not open UDP on ens18")
	}
	if !ens20 {
		t.Fatal("missing vxlan input on underlay ens20")
	}
}

func TestBuildAutoInputRulesVXLANExtraUnderlay(t *testing.T) {
	vpn := AutoInputVPN{VXLAN: []VXLANAutoEndpoint{
		{ID: "vxlan-a", Iface: "vxlan100", Port: 4789, Remote: "198.51.100.20", Underlay: "ens21"},
	}}
	rules := BuildAutoInputRules([]string{"ens18"}, "9443", vpn)
	var found bool
	for _, r := range rules {
		if r.ID == "auto-input-vxlan-vxlan-a-ens21" && r.Iif == "ens21" {
			found = true
		}
	}
	if !found {
		t.Fatalf("missing extra-underlay rule: %+v", rules)
	}
}

func TestBuildAutoVXLANForwardFilterRules(t *testing.T) {
	rules := BuildAutoVXLANForwardFilterRules([]VXLANAutoEndpoint{
		{ID: "vxlan-a", Iface: "vxlan100"},
	}, "ens19")
	if len(rules) != 2 {
		t.Fatalf("want 2, got %d", len(rules))
	}
	if rules[0].Iif != "ens19" || rules[0].Oif != "vxlan100" || !rules[0].System {
		t.Fatalf("out: %+v", rules[0])
	}
	if rules[1].Iif != "vxlan100" || rules[1].Oif != "ens19" {
		t.Fatalf("in: %+v", rules[1])
	}
}

func TestSyncAutoFilterRulesVXLAN(t *testing.T) {
	vpn := AutoInputVPN{VXLAN: []VXLANAutoEndpoint{
		{ID: "vxlan-a", Iface: "vxlan100", Port: 4789, Remote: "198.51.100.20"},
	}}
	merged, changed := SyncAutoFilterRules(nil, []string{"ens18"}, "9443", vpn, nil, LVSState{}, "ens19", "ens18", HairpinAddrResolver{})
	if !changed {
		t.Fatal("expected change")
	}
	var in, fwd bool
	for _, r := range merged {
		if r.ID == "auto-input-vxlan-vxlan-a-ens18" {
			in = true
		}
		if r.ID == "auto-fwd-vxlan-vxlan-a-out" {
			fwd = true
		}
	}
	if !in || !fwd {
		t.Fatalf("missing auto vxlan rules in %+v", ids(merged))
	}
}

func ids(rules []FilterRule) []string {
	out := make([]string, len(rules))
	for i, r := range rules {
		out[i] = r.ID
	}
	return out
}
