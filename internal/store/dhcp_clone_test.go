package store

import "testing"

func TestCloneDHCP(t *testing.T) {
	src := DefaultDHCP()
	src.Enabled = true
	src.DNSServers = []string{"1.1.1.1"}
	src.UpstreamDNS = []string{"8.8.8.8"}
	src.StaticLeases = []DHCPStaticLease{{MAC: "aa:bb:cc:dd:ee:ff", IP: "192.168.1.50"}}
	cl := CloneDHCP(src)
	src.DNSServers[0] = "9.9.9.9"
	src.StaticLeases[0].IP = "192.168.1.99"
	if cl.DNSServers[0] != "1.1.1.1" || cl.StaticLeases[0].IP != "192.168.1.50" {
		t.Fatalf("clone not independent: dns=%v lease=%v", cl.DNSServers, cl.StaticLeases)
	}
}
