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

func TestNormalizeLVSTcpUDP(t *testing.T) {
	l := LVSState{
		Enabled: true,
		VirtualServers: []LVSVirtualServer{{
			VIP: "203.0.113.10", Port: 443, Protocol: "tcp_udp",
			RealServers: []LVSRealServer{{IP: "10.0.0.10"}},
		}},
	}
	if err := NormalizeLVS(&l, "eth0"); err != nil {
		t.Fatal(err)
	}
	if l.VirtualServers[0].Protocol != "tcp_udp" {
		t.Fatalf("protocol=%q", l.VirtualServers[0].Protocol)
	}
}

func TestBuildLVSOCServCluster(t *testing.T) {
	vs, err := BuildLVSOCServCluster("203.0.113.10", 0, []string{"10.0.0.10", "10.0.0.11"}, "eth0", true, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if vs.Protocol != "tcp_udp" || vs.Service != "ocserv" || vs.Port != 443 {
		t.Fatalf("vs=%+v", vs)
	}
	if len(vs.RealServers) != 2 || vs.PersistenceSec != 3600 || vs.Scheduler != "sh" {
		t.Fatalf("vs=%+v", vs)
	}
}

func TestCollectLVSInputEndpoints(t *testing.T) {
	eps := CollectLVSInputEndpoints(LVSState{
		Enabled: true,
		VirtualServers: []LVSVirtualServer{{
			ID: "lvs-1", VIP: "203.0.113.10", Port: 443, Protocol: "tcp_udp",
			RealServers: []LVSRealServer{{IP: "10.0.0.10"}},
		}},
	})
	if len(eps) != 2 {
		t.Fatalf("endpoints=%d", len(eps))
	}
}
