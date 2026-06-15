package store

import "testing"

func TestLVSPersistenceSecOCServ(t *testing.T) {
	vs := LVSVirtualServer{Service: "ocserv", PersistenceSec: 3600}
	if LVSPersistenceSec(vs, "tcp") != 3600 {
		t.Fatal("tcp should persist")
	}
	if LVSPersistenceSec(vs, "udp") != 0 {
		t.Fatal("ocserv udp should not persist by default")
	}
	vs.PersistenceUDPSec = 600
	if LVSPersistenceSec(vs, "udp") != 600 {
		t.Fatal("explicit udp persistence")
	}
}

func TestBuildAutoLVSForwardFilterRules(t *testing.T) {
	rules := BuildAutoLVSForwardFilterRules(LVSState{
		Enabled: true,
		VirtualServers: []LVSVirtualServer{{
			ID: "lvs-1", VIP: "203.0.113.10", Port: 443, Protocol: "tcp_udp",
			WANDevice: "wan0", Service: "ocserv",
			RealServers: []LVSRealServer{{IP: "10.0.0.10", Port: 443}, {IP: "10.0.0.11"}},
		}},
	}, "lan0", "wan0")
	if len(rules) != 4 {
		t.Fatalf("rules=%d", len(rules))
	}
	var tcp, udp bool
	for _, r := range rules {
		if r.ID == AutoLVSForwardRuleID("lvs-1", "10.0.0.10", "tcp") && r.Iif == "wan0" && r.Oif == "lan0" {
			tcp = true
		}
		if r.Proto == "udp" && r.DstAddr == "10.0.0.11/32" {
			udp = true
		}
	}
	if !tcp || !udp {
		t.Fatalf("missing fwd rules: %v", rules)
	}
}
