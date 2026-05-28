package store

import "testing"

func TestSyncAutoFilterRules(t *testing.T) {
	vpn := AutoInputVPN{OCServEnabled: true, OCServTCP: 443, OCServUDP: 443, WGPorts: []int{51820}}
	user := FilterRule{ID: "fr-1", Chain: "forward", Action: "drop", Enabled: true}
	wanDevs := []string{"eth0"}
	merged, changed := SyncAutoFilterRules([]FilterRule{user}, wanDevs, "8443", vpn)
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
	_, changed2 := SyncAutoFilterRules(merged, wanDevs, "8443", vpn)
	if changed2 {
		t.Fatal("second sync should be no-op")
	}
}

func TestBuildAutoInputRulesAdminPort(t *testing.T) {
	rules := BuildAutoInputRules([]string{"wan0"}, "9090", AutoInputVPN{})
	if len(rules) < 2 {
		t.Fatal("expected admin + wan drop")
	}
	if rules[0].DstPort != 9090 {
		t.Fatalf("admin port: got %d", rules[0].DstPort)
	}
	if rules[0].ID != "auto-input-admin-wan0" {
		t.Fatalf("id: %s", rules[0].ID)
	}
}

func TestCollectWanInputDevices(t *testing.T) {
	st := DefaultState()
	st.Network.WanLinks = []WanLink{
		{ID: "w2", Device: "ens20", Enabled: true},
		{ID: "w3", Device: "ens19", Enabled: false},
		{ID: "w4", Device: "ens19", Enabled: true},
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
