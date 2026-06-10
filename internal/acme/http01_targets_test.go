package acme

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/netif"
)

func TestResolveHTTP01LocalIPsRequiresLocalIP(t *testing.T) {
	local, err := netif.CollectLocalGlobalIPv4()
	if err != nil {
		t.Skip(err)
	}
	if len(local) == 0 {
		t.Skip("no local global IPv4")
	}
	ip := local[0]
	got, err := ResolveHTTP01LocalIPs(ip)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != ip {
		t.Fatalf("got %v want [%s]", got, ip)
	}
	if _, err := ResolveHTTP01LocalIPs("203.0.113.199"); err == nil {
		t.Fatal("expected error for non-local IP")
	}
}
