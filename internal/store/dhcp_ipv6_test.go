package store

import "testing"

func TestNormalizeDHCPv6Prefix(t *testing.T) {
	d := &DHCPState{
		Enabled:     true,
		Interface:   "ens19",
		RangeStart:  "192.168.1.100",
		RangeEnd:    "192.168.1.200",
		Router:      "192.168.1.1",
		IPv6Enabled: true,
		IPv6Prefix:  "2001:db8::/64",
		IPv6Start:   "2001:db8::100",
		IPv6End:     "2001:db8::200",
	}
	if err := NormalizeDHCP(d, "ens19"); err != nil {
		t.Fatal(err)
	}
}

func TestNormalizeDHCPv6BadPrefix(t *testing.T) {
	d := &DHCPState{
		Enabled:     true,
		Interface:   "ens19",
		RangeStart:  "10.0.0.1",
		RangeEnd:    "10.0.0.2",
		Router:      "10.0.0.1",
		IPv6Enabled: true,
		IPv6Prefix:  "not-a-prefix",
		IPv6Start:   "2001:db8::1",
		IPv6End:     "2001:db8::2",
	}
	if err := NormalizeDHCP(d, "ens19"); err == nil {
		t.Fatal("expected error")
	}
}
