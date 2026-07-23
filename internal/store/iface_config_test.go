package store

import "testing"

func TestIfaceConfigUpsertFindRemove(t *testing.T) {
	st := &State{}
	up := true
	dhcp := false
	UpsertIfaceConfig(st, "ens18", []string{"10.0.0.1/24", "10.0.0.2/32"}, &up, &dhcp, nil, nil)
	if len(st.Network.Ifaces) != 1 {
		t.Fatalf("len=%d", len(st.Network.Ifaces))
	}
	ic, ok := FindIfaceConfig(*st, "ens18")
	if !ok || len(ic.IPv4) != 2 || ic.IPv4[0] != "10.0.0.1/24" || !ic.Up || ic.DHCP4 {
		t.Fatalf("find=%v %+v", ok, ic)
	}
	UpsertIfaceConfig(st, "ens18", []string{"192.168.1.1/24"}, nil, nil, nil, nil)
	ic, _ = FindIfaceConfig(*st, "ens18")
	if len(ic.IPv4) != 1 || ic.IPv4[0] != "192.168.1.1/24" {
		t.Fatalf("update ipv4=%v", ic.IPv4)
	}
	dhcpOn := true
	UpsertIfaceConfig(st, "ens18", nil, nil, &dhcpOn, nil, nil)
	ic, _ = FindIfaceConfig(*st, "ens18")
	if !ic.DHCP4 || len(ic.IPv4) != 1 {
		t.Fatalf("dhcp4 keep ipv4: %+v", ic)
	}
	gw := "192.168.1.254"
	pr := true
	UpsertIfaceConfig(st, "ens18", nil, nil, nil, &gw, &pr)
	ic, _ = FindIfaceConfig(*st, "ens18")
	if ic.Gateway != gw || !ic.PolicyRouting {
		t.Fatalf("gateway/policy: %+v", ic)
	}
	if !RemoveIfaceConfig(st, "ens18") {
		t.Fatal("remove expected true")
	}
	if _, ok := FindIfaceConfig(*st, "ens18"); ok {
		t.Fatal("should be gone")
	}
	if RemoveIfaceConfig(st, "ens18") {
		t.Fatal("second remove expected false")
	}
}
