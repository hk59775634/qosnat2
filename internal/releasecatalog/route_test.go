package releasecatalog

import "testing"

func TestURLsForRoute(t *testing.T) {
	direct := "https://github.com/hk59775634/qosnat2/releases/download/v2026052801/qosnat2-linux-amd64.tar.gz"
	v4 := URLsForRoute(direct, RouteGHProxyV4)
	if len(v4) != 1 || v4[0] != "https://v4.gh-proxy.org/"+direct {
		t.Fatalf("v4: %v", v4)
	}
	cdn := URLsForRoute(direct, RouteGHProxyCDN)
	if len(cdn) != 1 || cdn[0] != "https://cdn.gh-proxy.org/"+direct {
		t.Fatalf("cdn: %v", cdn)
	}
	d := URLsForRoute(direct, RouteDirect)
	if len(d) != 1 || d[0] != direct {
		t.Fatalf("direct: %v", d)
	}
}

func TestUsesWanEgress(t *testing.T) {
	if !UsesWanEgress(RouteWan1) || !UsesWanEgress(RouteWan2) {
		t.Fatal("wan routes should use egress")
	}
	if UsesWanEgress(RouteDirect) {
		t.Fatal("direct should not")
	}
}
