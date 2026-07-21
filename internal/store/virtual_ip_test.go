package store

import "testing"

func TestNormalizeVirtualIP(t *testing.T) {
	v := VirtualIP{Interface: "ens18", Address: "203.0.113.10", Enabled: true}
	if err := NormalizeVirtualIP(&v); err != nil {
		t.Fatal(err)
	}
	if v.Type != VirtualIPTypeIPAlias || v.Address != "203.0.113.10/32" || v.ID == "" {
		t.Fatalf("%+v", v)
	}
	v2 := VirtualIP{Interface: "ens18", Address: "203.0.113.11/32"}
	if err := NormalizeVirtualIP(&v2); err != nil {
		t.Fatal(err)
	}
	if VirtualIPHost(v2) != "203.0.113.11" {
		t.Fatalf("host=%s", VirtualIPHost(v2))
	}
	bad := VirtualIP{Interface: "ens18", Address: "not-an-ip"}
	if err := NormalizeVirtualIP(&bad); err == nil {
		t.Fatal("expected error")
	}
}
