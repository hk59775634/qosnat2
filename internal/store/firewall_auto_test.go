package store

import "testing"

func TestSyncAutoFilterRules(t *testing.T) {
	vpn := AutoInputVPN{OCServEnabled: true, OCServTCP: 443, OCServUDP: 443, WGPorts: []int{51820}}
	user := FilterRule{ID: "fr-1", Chain: "forward", Action: "drop", Enabled: true}
	wanDevs := []string{"eth0"}
	merged, changed := SyncAutoFilterRules([]FilterRule{user}, wanDevs, "8443", vpn, nil, LVSState{}, "br-lan", "eth0", HairpinAddrResolver{})
	if !changed {
		t.Fatal("expected change on first sync")
	}
	if len(merged) < 3 {
		t.Fatalf("expected user + auto rules, got %d", len(merged))
	}
	if merged[0].ID != "fr-1" {
		t.Fatal("user rule should stay first")
	}
	if !IsAutoManagedRule(merged[len(merged)-1]) {
		t.Fatal("last rule should be auto wan drop")
	}
	if merged[len(merged)-1].ID != "auto-input-wan-drop-eth0" {
		t.Fatalf("unexpected last id: %s", merged[len(merged)-1].ID)
	}
	_, changed2 := SyncAutoFilterRules(merged, wanDevs, "8443", vpn, nil, LVSState{}, "br-lan", "eth0", HairpinAddrResolver{})
	if changed2 {
		t.Fatal("second sync should be no-op")
	}
}

func TestSyncAutoFilterRulesInputOrder(t *testing.T) {
	userDrop := FilterRule{ID: "fr-wan-drop", Chain: "input", Action: "drop", Iif: "eth0", Enabled: true}
	merged, _ := SyncAutoFilterRules([]FilterRule{userDrop}, []string{"eth0"}, "8443", AutoInputVPN{}, nil, LVSState{}, "br-lan", "eth0", HairpinAddrResolver{})
	idx := func(id string) int {
		for i, r := range merged {
			if r.ID == id {
				return i
			}
		}
		return -1
	}
	admin := idx("auto-input-admin-eth0")
	user := idx("fr-wan-drop")
	wanDrop := idx("auto-input-wan-drop-eth0")
	if admin < 0 || user < 0 || wanDrop < 0 {
		t.Fatalf("missing rules in %v", merged)
	}
	if !(admin < user && user < wanDrop) {
		t.Fatalf("want admin < user drop < auto wan drop, got admin=%d user=%d wanDrop=%d", admin, user, wanDrop)
	}
}

func TestBuildAutoInputRulesSNMP(t *testing.T) {
	rules := BuildAutoInputRules([]string{"wan0"}, "8080", AutoInputVPN{
		SNMPEnabled:         true,
		SNMPPort:            161,
		SNMPAllowedNetworks: []string{"10.0.0.0/8"},
	})
	found := false
	for _, r := range rules {
		if r.ID == "auto-input-snmp-161-wan0-0" && r.Proto == "udp" && r.DstPort == 161 && r.SrcAddr == "10.0.0.0/8" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("missing snmp auto rule: %v", rules)
	}
}

func TestBuildAutoInputRulesLVS(t *testing.T) {
	rules := BuildAutoInputRules([]string{"wan0"}, "8080", AutoInputVPN{
		LVSEndpoints: []AutoInputLVSEndpoint{
			{VSID: "lvs-1", VIP: "203.0.113.10", Port: 443, Proto: "tcp"},
			{VSID: "lvs-1", VIP: "203.0.113.10", Port: 443, Proto: "udp"},
		},
	})
	var tcp, udp bool
	for _, r := range rules {
		if r.ID == "auto-input-lvs-lvs-1-tcp-wan0" && r.DstAddr == "203.0.113.10/32" && r.DstPort == 443 {
			tcp = true
		}
		if r.ID == "auto-input-lvs-lvs-1-udp-wan0" && r.Proto == "udp" {
			udp = true
		}
	}
	if !tcp || !udp {
		t.Fatalf("missing lvs rules tcp=%v udp=%v rules=%v", tcp, udp, rules)
	}
}

func TestBuildAutoInputRulesAdminPort(t *testing.T) {
	rules := BuildAutoInputRules([]string{"wan0"}, "9090", AutoInputVPN{})
	if len(rules) < 3 {
		t.Fatal("expected admin + ssh + wan drop")
	}
	if rules[0].DstPort != 9090 {
		t.Fatalf("admin port: got %d", rules[0].DstPort)
	}
	if rules[0].ID != "auto-input-admin-wan0" {
		t.Fatalf("id: %s", rules[0].ID)
	}
	if rules[1].ID != "auto-input-ssh-wan0" || rules[1].DstPort != DefaultSSHPort || rules[1].Proto != "tcp" {
		t.Fatalf("ssh rule: %+v", rules[1])
	}
}

func TestBuildAutoInputRulesSSHSkippedWhenAdminIs22(t *testing.T) {
	rules := BuildAutoInputRules([]string{"wan0"}, "22", AutoInputVPN{})
	for _, r := range rules {
		if r.ID == "auto-input-ssh-wan0" {
			t.Fatal("ssh auto rule must be omitted when admin port is already 22")
		}
	}
	if rules[0].ID != "auto-input-admin-wan0" || rules[0].DstPort != 22 {
		t.Fatalf("admin should cover ssh: %+v", rules[0])
	}
}

func TestCollectWanInputDevices(t *testing.T) {
	st := DefaultState()
	st.Network.WanLinks = []WanLink{
		{ID: "w2", Device: "ens20", Enabled: true},
		{ID: "w3", Device: "ens19", Enabled: false},
		{ID: "w4", Device: "ens19", Enabled: true},
		{ID: "wan-warp", Device: "qwp0", Enabled: true, WarpManaged: true},
		{ID: "wan-proxy-x", Device: "qpe0", Enabled: true, ProxyManaged: true},
	}
	devs := CollectWanInputDevices("ens18", "ens19", st)
	want := []string{"ens18", "ens20"}
	if len(devs) != len(want) {
		t.Fatalf("got %v want %v", devs, want)
	}
	for i, w := range want {
		if devs[i] != w {
			t.Fatalf("got %v want %v", devs, want)
		}
	}
}

func TestCollectWanForwardDevicesIncludesWanLinkWithoutEgress(t *testing.T) {
	st := DefaultState()
	st.Network.WanLinks = []WanLink{
		{ID: "w2", Device: "ens20", Enabled: true},
	}
	devs := CollectWanForwardDevices("ens18", "ens19", st, nil)
	if len(devs) != 2 {
		t.Fatalf("got %v want ens18+ens20", devs)
	}
}

func TestBuildAutoInputRulesMultiWAN(t *testing.T) {
	vpn := AutoInputVPN{WGPorts: []int{51820}}
	rules := BuildAutoInputRules([]string{"ens18", "ens20"}, "9443", vpn)
	var drops int
	for _, r := range rules {
		if r.Action == "drop" && r.Iif == "ens20" {
			drops++
		}
		if r.Iif == "ens20" && r.Proto == "udp" && r.DstPort == 51820 {
			// wg on secondary wan
		}
	}
	if drops != 1 {
		t.Fatalf("expected one ens20 drop, got %d rules", len(rules))
	}
	admin := 0
	for _, r := range rules {
		if r.ID == "auto-input-admin-ens18" || r.ID == "auto-input-admin-ens20" {
			admin++
		}
	}
	if admin != 2 {
		t.Fatalf("expected admin on both WANs, got %d", admin)
	}
}
