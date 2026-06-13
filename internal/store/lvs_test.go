package store

import "testing"

func TestNormalizeLVS(t *testing.T) {
	l := LVSState{
		Enabled: true,
		Mode:    "nat",
		VirtualServers: []LVSVirtualServer{{
			VIP:      "203.0.113.10",
			Port:     80,
			Protocol: "tcp",
			RealServers: []LVSRealServer{{
				IP: "10.0.0.10",
			}},
		}},
	}
	if err := NormalizeLVS(&l, "eth0"); err != nil {
		t.Fatal(err)
	}
	if l.VirtualServers[0].Scheduler != "rr" {
		t.Fatalf("scheduler=%q", l.VirtualServers[0].Scheduler)
	}
	if l.VirtualServers[0].RealServers[0].Port != 80 {
		t.Fatalf("rs port=%d", l.VirtualServers[0].RealServers[0].Port)
	}
}

func TestLVSVSConflictsForward(t *testing.T) {
	vs := LVSVirtualServer{VIP: "203.0.113.10", Port: 443, Protocol: "tcp"}
	fwd := []WanPortForward{{
		IPVersion: "ipv4",
		Proto:     "tcp",
		DstPort:   443,
		DstAddr:   "203.0.113.10/32",
	}}
	if !LVSVSConflictsForward(vs, fwd) {
		t.Fatal("expected conflict")
	}
}
