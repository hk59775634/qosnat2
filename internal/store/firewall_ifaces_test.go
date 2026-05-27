package store

import "testing"

func TestBuildFirewallIfaceList(t *testing.T) {
	st := DefaultState()
	st.Network.WanLinks = []WanLink{
		{ID: "wan2", Name: "US", Device: "ens20", Enabled: true},
	}
	st.Network.VLANs = []VLANIface{{Parent: "eth0", VID: 100, Name: "eth0.100"}}
	list := BuildFirewallIfaceList(st, "br-lan", "eth1", []string{"eth1", "ens20", "eth2"})
	if len(list) < 3 {
		t.Fatalf("expected lan+wan+extra, got %d", len(list))
	}
	if list[0].Role != "LAN" || list[0].Name != "br-lan" {
		t.Fatalf("LAN first: %+v", list[0])
	}
	names := map[string]bool{}
	for _, x := range list {
		names[x.Name] = true
	}
	if !names["ens20"] || !names["eth0.100"] {
		t.Fatalf("missing wan2 or vlan: %+v", list)
	}
}
