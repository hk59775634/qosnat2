package store

import "testing"

func TestNormalizeWanForwardLegacy(t *testing.T) {
	f := WanPortForward{
		Proto:    "tcp",
		WanPort:  8080,
		HostIP:   "10.0.0.5",
		HostPort: 80,
		Comment:  "test",
	}
	if err := NormalizeWanForward(&f, "ens18"); err != nil {
		t.Fatal(err)
	}
	if f.DstPort != 8080 || f.RedirectIP != "10.0.0.5" || f.RedirectPort != 80 {
		t.Fatalf("migrate failed: %+v", f)
	}
	if f.Interface != "ens18" || f.SrcAddr != "0.0.0.0/0" {
		t.Fatalf("defaults: %+v", f)
	}
}

func TestForwardProtos(t *testing.T) {
	if len(ForwardProtos("tcp_udp")) != 2 {
		t.Fatal("tcp_udp")
	}
}
