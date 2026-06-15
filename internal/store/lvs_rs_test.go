package store

import "testing"

func TestNormalizeLVSRS(t *testing.T) {
	l := LVSState{
		Role:    LVSRoleRS,
		Enabled: true,
		RS: LVSRSConfig{
			Entries: []LVSRSBinding{{
				VIP: "203.0.113.10", Port: 443, Protocol: "tcp_udp",
			}},
		},
	}
	if err := NormalizeLVS(&l, "eth0"); err != nil {
		t.Fatal(err)
	}
	if l.Mode != "dr" {
		t.Fatalf("mode=%q want dr", l.Mode)
	}
	if len(l.RS.Entries) != 1 || l.RS.Entries[0].Protocol != "tcp_udp" {
		t.Fatalf("entries=%+v", l.RS.Entries)
	}
}

func TestNormalizeLVSRSRequiresEntries(t *testing.T) {
	l := LVSState{Role: LVSRoleRS, Enabled: true}
	if err := NormalizeLVS(&l, "eth0"); err == nil {
		t.Fatal("expected error")
	}
}

func TestCollectLVSRSInputEndpoints(t *testing.T) {
	eps := CollectLVSRSInputEndpoints(LVSState{
		Role:    LVSRoleRS,
		Enabled: true,
		RS: LVSRSConfig{Entries: []LVSRSBinding{{
			ID: "rs-1", VIP: "203.0.113.10", Port: 443, Protocol: "tcp_udp",
		}}},
	})
	if len(eps) != 2 {
		t.Fatalf("got %d endpoints", len(eps))
	}
}

func TestBuildAutoLVSRSInputRules(t *testing.T) {
	rules := BuildAutoLVSRSInputRules(LVSState{
		Role:    LVSRoleRS,
		Enabled: true,
		RS: LVSRSConfig{Entries: []LVSRSBinding{{
			ID: "rs-1", VIP: "203.0.113.10", Port: 10443, Protocol: "tcp",
		}}},
	}, "lan0")
	if len(rules) != 1 {
		t.Fatalf("got %d rules", len(rules))
	}
	r := rules[0]
	if r.Iif != "lan0" || r.DstAddr != "203.0.113.10/32" || r.DstPort != 10443 {
		t.Fatalf("rule=%+v", r)
	}
}

func TestCollectLVSInputEndpointsDirectorOnly(t *testing.T) {
	l := LVSState{
		Role:    LVSRoleRS,
		Enabled: true,
		RS: LVSRSConfig{Entries: []LVSRSBinding{{VIP: "1.2.3.4", Port: 80}}},
	}
	if eps := CollectLVSInputEndpoints(l); len(eps) != 0 {
		t.Fatal("director endpoints should be empty for rs role")
	}
}
