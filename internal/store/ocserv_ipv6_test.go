package store

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/linknet"
)

func TestValidateOCServIPv6Pool(t *testing.T) {
	if err := ValidateOCServIPv6Pool("", 0, 0); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOCServIPv6Pool("", 0, 128); err == nil {
		t.Fatal("expected error for subnet without network")
	}
	if err := ValidateOCServIPv6Pool("fd12:198:18:250::/64", 0, 128); err != nil {
		t.Fatal(err)
	}
	if err := ValidateOCServIPv6Pool("fd12:198:18:250::/64", 0, 48); err == nil {
		t.Fatal("subnet shorter than pool should fail")
	}
	if err := ValidateOCServIPv6Pool("10.0.0.0/24", 0, 128); err == nil {
		t.Fatal("ipv4 must fail")
	}
}

func TestNormalizeOCServIPv6(t *testing.T) {
	o := DefaultOCServ()
	o.IPv6Network = "fd12:198:18:250::"
	o.IPv6SubnetPrefix = 0
	if err := NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	if o.IPv6Network != "fd12:198:18:250::/64" {
		t.Fatalf("got network %q", o.IPv6Network)
	}
	if o.IPv6SubnetPrefix != linknet.OCServDefaultIPv6SubnetPrefix {
		t.Fatalf("got subnet %d", o.IPv6SubnetPrefix)
	}
}

func TestCollectOCServIPv6Pools(t *testing.T) {
	o := DefaultOCServ()
	o.Enabled = true
	o.IPv6Network = "fd12:198:18:250::/64"
	o.Groups = []OCServGroup{{Name: "g1", IPv6Network: "fd12:198:18:251::/64"}}
	o.Vhosts = []OCServVhost{
		{Enabled: true, Domain: "a.example", IPv6Network: "fd12:198:18:252::/64"},
		{Enabled: false, Domain: "b.example", IPv6Network: "fd12:198:18:253::/64"},
	}
	got := CollectOCServIPv6Pools(o)
	want := map[string]bool{
		"fd12:198:18:250::/64": true,
		"fd12:198:18:251::/64": true,
		"fd12:198:18:252::/64": true,
	}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for _, c := range got {
		delete(want, c)
	}
	if len(want) > 0 {
		t.Fatalf("missing %v", want)
	}
}

func TestVhostFromGlobalCopiesIPv6(t *testing.T) {
	o := DefaultOCServ()
	o.IPv6Network = linknet.OCServDefaultIPv6Network
	o.IPv6SubnetPrefix = 128
	v := VhostFromGlobal(o, "vpn.example.com", "", "")
	if v.IPv6Network != o.IPv6Network || v.IPv6SubnetPrefix != 128 {
		t.Fatalf("%+v", v)
	}
}
