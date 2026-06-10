package store

import "testing"

func TestNormalizeDynamicRoutingBGP(t *testing.T) {
	dr := DynamicRoutingState{
		BGP: BGPConfig{
			Enabled:  true,
			ASN:      65001,
			RouterID: "1.1.1.1",
			Neighbors: []BGPNeighbor{{
				Address:   "10.0.0.2",
				RemoteASN: 65002,
				Enabled:   true,
			}},
			Networks: []string{"10.0.0.0/24"},
		},
	}
	if err := NormalizeDynamicRouting(&dr); err != nil {
		t.Fatal(err)
	}
	if dr.BGP.Neighbors[0].ID == "" {
		t.Fatal("expected neighbor id")
	}
}

func TestNormalizeDynamicRoutingBGPInvalid(t *testing.T) {
	dr := DynamicRoutingState{
		BGP: BGPConfig{
			Enabled: true,
			ASN:     65001,
			Neighbors: []BGPNeighbor{{
				Address: "10.0.0.2",
				Enabled: true,
			}},
		},
	}
	if err := NormalizeDynamicRouting(&dr); err == nil {
		t.Fatal("expected remote_asn error")
	}
}

func TestNormalizeDynamicRoutingOSPF(t *testing.T) {
	dr := DynamicRoutingState{
		OSPF: OSPFConfig{
			Enabled:  true,
			RouterID: "2.2.2.2",
			Networks: []OSPFNetwork{{
				Prefix:  "192.168.1.0/24",
				Area:    "0",
				Enabled: true,
			}},
		},
	}
	if err := NormalizeDynamicRouting(&dr); err != nil {
		t.Fatal(err)
	}
	if dr.OSPF.Networks[0].Area != "0.0.0.0" {
		t.Fatalf("area=%q", dr.OSPF.Networks[0].Area)
	}
}
