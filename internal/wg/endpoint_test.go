package wg

import "testing"

func TestParseEndpointHost(t *testing.T) {
	cases := map[string]string{
		"219.130.112.106:5060": "219.130.112.106",
		"219.130.112.106":      "219.130.112.106",
		"[2001:db8::1]:51820":  "2001:db8::1",
		"":                     "",
	}
	for in, want := range cases {
		if got := ParseEndpointHost(in); got != want {
			t.Fatalf("%q: got %q want %q", in, got, want)
		}
	}
}

func TestEndpointLooksTunnelPinned(t *testing.T) {
	allowed := []string{"100.64.1.2/32", "192.168.5.0/24"}
	if !EndpointLooksTunnelPinned("100.64.1.2:5060", allowed) {
		t.Fatal("tunnel /32 should pin-detect")
	}
	if !EndpointLooksTunnelPinned("192.168.5.9:51820", allowed) {
		t.Fatal("allowed LAN should pin-detect")
	}
	if EndpointLooksTunnelPinned("219.130.112.106:5060", allowed) {
		t.Fatal("public endpoint must not look tunnel-pinned")
	}
}

func TestShouldRepinEndpoint(t *testing.T) {
	allowed := []string{"100.64.1.2/32", "192.168.5.0/24"}
	cfg := "219.130.112.106:5060"
	if ShouldRepinEndpoint(cfg, cfg, allowed) {
		t.Fatal("equal should not repin")
	}
	if !ShouldRepinEndpoint(cfg, "100.64.1.2:5060", allowed) {
		t.Fatal("tunnel drift should repin")
	}
	if !ShouldRepinEndpoint(cfg, "203.0.113.9:5060", allowed) {
		t.Fatal("static config should repin any drift")
	}
	if !ShouldRepinEndpoint(cfg, "", allowed) {
		t.Fatal("empty runtime should repin")
	}
	if ShouldRepinEndpoint("", "1.2.3.4:51820", allowed) {
		t.Fatal("no config endpoint means roaming OK")
	}
}
