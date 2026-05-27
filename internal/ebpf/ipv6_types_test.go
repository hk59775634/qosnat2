package ebpf

import "testing"

func TestIPToLPMKeyV6(t *testing.T) {
	k, err := IPToLPMKeyV6("fd00:1111:2222::/48")
	if err != nil {
		t.Fatal(err)
	}
	if k.Prefixlen != 48 {
		t.Fatalf("prefixlen %d", k.Prefixlen)
	}
	if len(k.Marshal()) != 20 {
		t.Fatalf("marshal len %d", len(k.Marshal()))
	}
}

func TestProfileMapForCIDR(t *testing.T) {
	v4, v6, err := profileMapForCIDR("10.0.0.0/8")
	if err != nil || !v4 || v6 {
		t.Fatalf("v4=%v v6=%v err=%v", v4, v6, err)
	}
	v4, v6, err = profileMapForCIDR("fd00::/48")
	if err != nil || v4 || !v6 {
		t.Fatalf("v6 v4=%v v6=%v err=%v", v4, v6, err)
	}
}
