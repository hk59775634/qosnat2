package linknet

import "testing"

func TestProxyTunHostCIDR(t *testing.T) {
	if ProxyTunHostCIDR(0) != "198.18.1.1/30" {
		t.Fatal(ProxyTunHostCIDR(0))
	}
	if ProxyTunHostCIDR(5) != "198.18.6.1/30" {
		t.Fatal(ProxyTunHostCIDR(5))
	}
	if ProxyTunHostCIDR(-1) != "" || ProxyTunHostCIDR(64) != "" {
		t.Fatal("bounds")
	}
}

func TestOCServDefaults(t *testing.T) {
	if OCServDefaultIPv4Network != "198.18.250.0" || OCServDefaultIPv4CIDR != "198.18.250.0/24" {
		t.Fatal(OCServDefaultIPv4Network, OCServDefaultIPv4CIDR)
	}
}

func TestWireGuardAddressHelpers(t *testing.T) {
	if WireGuardHostCIDR(0) != "198.19.0.1/24" || WireGuardHostCIDR(3) != "198.19.3.1/24" {
		t.Fatal(WireGuardHostCIDR(0), WireGuardHostCIDR(3))
	}
	if got := WireGuardSuggestPeerAllowedIP("198.19.0.1/24"); got != "198.19.0.10/32" {
		t.Fatalf("suggest=%s", got)
	}
	if got := WireGuardClientFallbackAddr("198.19.2.1/24"); got != "198.19.2.2/32" {
		t.Fatalf("fallback=%s", got)
	}
}
