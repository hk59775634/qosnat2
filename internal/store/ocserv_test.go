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
