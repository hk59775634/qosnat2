package store

import "testing"

func TestNormalizeDHCPLegacyDNSInference(t *testing.T) {
	d := DefaultDHCP()
	d.Enabled = true
	d.Interface = "eth0"
	d.DNSServers = []string{"8.8.8.8"}
	if err := NormalizeDHCP(&d, "eth0"); err != nil {
		t.Fatal(err)
	}
	if !d.DNSEnabled {
		t.Fatal("expected dns_enabled inferred from legacy dns_servers")
	}
}

func TestNormalizeDHCPDNSOnly(t *testing.T) {
	d := DefaultDHCP()
	d.DNSEnabled = true
	d.Interface = "eth0"
	d.UpstreamDNS = []string{"1.1.1.1"}
	if err := NormalizeDHCP(&d, "eth0"); err != nil {
		t.Fatal(err)
	}
	if d.Enabled {
		t.Fatal("dhcp should stay disabled")
	}
	if !d.ServiceActive() {
		t.Fatal("service should be active")
	}
}

func TestNormalizeDHCPBothOff(t *testing.T) {
	d := DefaultDHCP()
	if err := NormalizeDHCP(&d, "eth0"); err != nil {
		t.Fatal(err)
	}
	if d.ServiceActive() {
		t.Fatal("service should be inactive")
	}
}
