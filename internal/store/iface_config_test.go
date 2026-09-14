package store

import (
	"encoding/json"
	"testing"
)

func TestIfaceConfigUpsertFindRemove(t *testing.T) {
	st := &State{}
	up := true
	dhcp := false
	UpsertIfaceConfig(st, "ens18", []string{"10.0.0.1/24", "10.0.0.2/32"}, &up, &dhcp, nil, nil, nil, nil)
	if len(st.Network.Ifaces) != 1 {
		t.Fatalf("len=%d", len(st.Network.Ifaces))
	}
	ic, ok := FindIfaceConfig(*st, "ens18")
	if !ok || len(ic.IPv4) != 2 || ic.IPv4[0] != "10.0.0.1/24" || !ic.Up || ic.DHCP4 {
		t.Fatalf("find=%v %+v", ok, ic)
	}
	UpsertIfaceConfig(st, "ens18", []string{"192.168.1.1/24"}, nil, nil, nil, nil, nil, nil)
	ic, _ = FindIfaceConfig(*st, "ens18")
	if len(ic.IPv4) != 1 || ic.IPv4[0] != "192.168.1.1/24" {
		t.Fatalf("update ipv4=%v", ic.IPv4)
	}
	dhcpOn := true
	UpsertIfaceConfig(st, "ens18", nil, nil, &dhcpOn, nil, nil, nil, nil)
	ic, _ = FindIfaceConfig(*st, "ens18")
	if !ic.DHCP4 || len(ic.IPv4) != 1 {
		t.Fatalf("dhcp4 keep ipv4: %+v", ic)
	}
	gw := "192.168.1.254"
	pr := true
	UpsertIfaceConfig(st, "ens18", nil, nil, nil, &gw, &pr, nil, nil)
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

func TestIfaceConfigLFNUpsertAndLegacyJSON(t *testing.T) {
	st := &State{}
	up := true
	on := true
	mss := 1420
	UpsertIfaceConfig(st, "ens18", []string{"203.0.113.2/30"}, &up, nil, nil, nil, &on, &mss)
	ic, ok := FindIfaceConfig(*st, "ens18")
	if !ok || !ic.LfnEnabled || ic.LfnMssClamp != 1420 {
		t.Fatalf("lfn upsert: %+v ok=%v", ic, ok)
	}
	if !AnyLFNEnabled(*st) {
		t.Fatal("any lfn")
	}
	off := false
	UpsertIfaceConfig(st, "ens18", nil, nil, nil, nil, nil, &off, nil)
	ic, _ = FindIfaceConfig(*st, "ens18")
	if ic.LfnEnabled || ic.LfnMssClamp != 1420 {
		t.Fatalf("keep mss when toggling off: %+v", ic)
	}
	if AnyLFNEnabled(*st) {
		t.Fatal("last iface off")
	}

	raw := `{"network":{"ifaces":[{"device":"ens18","ipv4":["10.0.0.1/24"],"up":true}]}}`
	var disk State
	if err := json.Unmarshal([]byte(raw), &disk); err != nil {
		t.Fatal(err)
	}
	ic, ok = FindIfaceConfig(disk, "ens18")
	if !ok || ic.LfnEnabled || ic.LfnMssClamp != 0 {
		t.Fatalf("legacy missing lfn fields must be off: %+v", ic)
	}
}

func TestEffectiveLFNMSS(t *testing.T) {
	if EffectiveLFNMSS(false, 0, 1500) != 0 {
		t.Fatal("disabled")
	}
	if got := EffectiveLFNMSS(true, 0, 1500); got != DefaultLFNMSS {
		t.Fatalf("default mss=%d", got)
	}
	if got := EffectiveLFNMSS(true, 1420, 1500); got != 1420 {
		t.Fatalf("explicit=%d", got)
	}
	if got := EffectiveLFNMSS(true, 1420, 1280); got != 1240 {
		t.Fatalf("mtu clamp want 1240 got %d", got)
	}
	if err := ValidateLFNMSSClamp(0); err != nil {
		t.Fatal(err)
	}
	if err := ValidateLFNMSSClamp(100); err == nil {
		t.Fatal("too small")
	}
}
