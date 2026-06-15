package store

import "testing"

func TestNormalizeOCServ(t *testing.T) {
	o := DefaultOCServ()
	o.Users = []OCServUser{{Username: "alice", Password: "secret"}}
	if err := NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	if o.TCPPort != 443 || o.IPv4Network != "10.250.0.0" {
		t.Fatalf("%+v", o)
	}
}

func TestNormalizeOCServRadius(t *testing.T) {
	o := DefaultOCServ()
	o.AuthMethod = OCServAuthRadius
	o.Radius.Server = "radius.example.com"
	o.Radius.Secret = "shared"
	o.Users = []OCServUser{{Username: "x", Password: "y"}}
	if err := NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	if len(o.Users) != 0 {
		t.Fatal("radius mode should clear local users")
	}
	if o.Radius.AuthPort != 1812 {
		t.Fatalf("%+v", o.Radius)
	}
}

func TestNormalizeOCServRadiusEmptyPool(t *testing.T) {
	o := DefaultOCServ()
	o.AuthMethod = OCServAuthRadius
	o.Radius.Server = "radius.example.com"
	o.Radius.Secret = "shared"
	o.IPv4Network = ""
	o.IPv4Netmask = ""
	if err := NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	if o.IPv4Network != "" || o.IPv4Netmask != "" {
		t.Fatalf("radius empty pool should stay empty: %+v", o)
	}
}

func TestNormalizeOCServRadiusLocalPool(t *testing.T) {
	o := DefaultOCServ()
	o.AuthMethod = OCServAuthRadius
	o.Radius.Server = "radius.example.com"
	o.Radius.Secret = "shared"
	o.IPv4Network = "10.9.8.0"
	o.IPv4Netmask = "255.255.255.0"
	if err := NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	if o.IPv4Network != "10.9.8.0" {
		t.Fatalf("%+v", o)
	}
}

func TestMergeOCServAdvancedDefaults(t *testing.T) {
	o := DefaultOCServ()
	o.Advanced = OCServAdvanced{}
	if err := NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	if !o.Advanced.CiscoClientCompat || !o.Advanced.Tcp {
		t.Fatalf("%+v", o.Advanced)
	}
}
