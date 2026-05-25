package acme

import "testing"

func TestNormalizeDomain(t *testing.T) {
	d, err := NormalizeDomain("https://VPN.Example.COM:443/path")
	if err != nil || d != "vpn.example.com" {
		t.Fatalf("%q %v", d, err)
	}
	if _, err := NormalizeDomain(""); err == nil {
		t.Fatal("expected error")
	}
}
