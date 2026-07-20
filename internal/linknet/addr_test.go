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

func TestWarpVethInInternalRange(t *testing.T) {
	if WarpHostVethCIDR != "198.18.0.1/30" || WarpNSVethCIDR != "198.18.0.2/30" {
		t.Fatal(WarpHostVethCIDR, WarpNSVethCIDR)
	}
}
