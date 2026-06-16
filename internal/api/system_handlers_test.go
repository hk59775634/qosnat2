package api

import "testing"

func TestValidateHostname(t *testing.T) {
	for _, h := range []string{"gw", "router.local", "vpn-01.example.com"} {
		if err := validateHostname(h); err != nil {
			t.Fatalf("%q: %v", h, err)
		}
	}
	for _, h := range []string{"", "bad host", "a..b"} {
		if err := validateHostname(h); err == nil {
			t.Fatalf("%q should be invalid", h)
		}
	}
	if err := validateHostname(string(make([]byte, 254))); err == nil {
		t.Fatal("overlong hostname should be invalid")
	}
}
