package wg

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderConfDefaultRoutesAllPeers(t *testing.T) {
	body := RenderConf(store.WireGuardState{
		PrivateKey: "serverpriv",
		Address:    "198.19.0.1/24",
		ListenPort: 51820,
		Peers: []store.WGPeer{
			{PublicKey: "peer1", AllowedIPs: []string{"198.19.0.10/32"}},
		},
	})
	if strings.Contains(body, "Table = off") {
		t.Fatalf("default should use wg-quick auto routes: %s", body)
	}
	if strings.Contains(body, "PostUp") {
		t.Fatalf("unexpected PostUp: %s", body)
	}
	if !strings.Contains(body, "AllowedIPs = 198.19.0.10/32") {
		t.Fatalf("missing allowed: %s", body)
	}
}

func TestRenderConfSkipRoutePeer(t *testing.T) {
	route := false
	body := RenderConf(store.WireGuardState{
		PrivateKey: "serverpriv",
		Address:    "198.19.0.1/24",
		ListenPort: 51820,
		Peers: []store.WGPeer{
			{PublicKey: "peer-route", AllowedIPs: []string{"198.19.0.10/32"}},
			{PublicKey: "peer-noroute", AllowedIPs: []string{"10.99.0.0/24", "2001:db8::1/128"}, RouteAllowedIPs: &route},
		},
	})
	if !strings.Contains(body, "Table = off") {
		t.Fatalf("expected Table=off: %s", body)
	}
	if !strings.Contains(body, "PostUp = ip -4 route add 198.19.0.10/32 dev %i\n") {
		t.Fatalf("expected PostUp for routed peer only: %s", body)
	}
	post := body[strings.Index(body, "PostUp"):]
	if end := strings.Index(post, "\n"); end >= 0 {
		post = post[:end]
	}
	if strings.Contains(post, "10.99.0.0/24") || strings.Contains(post, "2001:db8") {
		t.Fatalf("no-route peer must not appear in PostUp: %s", post)
	}
	if !strings.Contains(body, "PreDown = ip -4 route del 198.19.0.10/32 dev %i") {
		t.Fatalf("expected PreDown: %s", body)
	}
	// AllowedIPs still present for crypto routing
	if !strings.Contains(body, "AllowedIPs = 10.99.0.0/24, 2001:db8::1/128") {
		t.Fatalf("crypto AllowedIPs missing: %s", body)
	}
}

func TestRenderConfAllPeersNoRoute(t *testing.T) {
	route := false
	body := RenderConf(store.WireGuardState{
		PrivateKey: "serverpriv",
		Address:    "198.19.0.1/24",
		Peers: []store.WGPeer{
			{PublicKey: "p1", AllowedIPs: []string{"198.19.0.10/32"}, RouteAllowedIPs: &route},
		},
	})
	if !strings.Contains(body, "Table = off") {
		t.Fatal("expected Table=off")
	}
	if strings.Contains(body, "PostUp") {
		t.Fatalf("no routes to install: %s", body)
	}
}

func TestNormalizeRouteCIDR(t *testing.T) {
	c, fam, ok := normalizeRouteCIDR("10.0.0.1")
	if !ok || c != "10.0.0.1/32" || fam != "-4" {
		t.Fatalf("got %q %q %v", c, fam, ok)
	}
	c, fam, ok = normalizeRouteCIDR("2001:db8::1/64")
	if !ok || fam != "-6" || !strings.HasPrefix(c, "2001:db8::") {
		t.Fatalf("got %q %q %v", c, fam, ok)
	}
	if _, _, ok := normalizeRouteCIDR("10.0.0.1; rm -rf /"); ok {
		t.Fatal("shell metachar must reject")
	}
}

func TestPeerRouteAllowedIPsDefault(t *testing.T) {
	if !store.PeerRouteAllowedIPs(store.WGPeer{}) {
		t.Fatal("nil pointer should default true")
	}
	f := false
	if store.PeerRouteAllowedIPs(store.WGPeer{RouteAllowedIPs: &f}) {
		t.Fatal("false should stick")
	}
}
