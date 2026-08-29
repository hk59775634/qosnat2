package ocserv

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderConfIPv6DualStack(t *testing.T) {
	o := store.DefaultOCServ()
	o.IPv6Network = "fd12:198:18:250::/64"
	o.IPv6SubnetPrefix = 128
	o.Routes = []string{"default"}
	o.Advanced.RxDataPerSec = 1250000
	o.Advanced.TxDataPerSec = 2500000
	if err := store.NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	conf := RenderConf(o, nil)
	for _, want := range []string{
		"ipv6-network = fd12:198:18:250::/64",
		"ipv6-subnet-prefix = 128",
		"route = default",
		"route = ::/0",
		"rx-data-per-sec = 1250000",
		"tx-data-per-sec = 2500000",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("missing %q in:\n%s", want, conf)
		}
	}
}

func TestRenderGroupConfIPv6(t *testing.T) {
	g := store.OCServGroup{
		Name:             "hk",
		IPv6Network:      "fd12:198:18:251::/64",
		IPv6SubnetPrefix: 128,
		Routes:           []string{"default"},
		RxDataPerSec:     1000,
	}
	conf := renderGroupConf(g)
	if !strings.Contains(conf, "ipv6-network = fd12:198:18:251::/64") {
		t.Fatalf("%s", conf)
	}
	if !strings.Contains(conf, "ipv6-subnet-prefix = 128") {
		t.Fatalf("%s", conf)
	}
	if !strings.Contains(conf, "route = ::/0") {
		t.Fatalf("expected auto ::/0:\n%s", conf)
	}
}

func TestRenderVhostIPv6SubnetPrefix(t *testing.T) {
	o := store.DefaultOCServ()
	o.Vhosts = []store.OCServVhost{{
		Enabled:          true,
		Domain:           "v.example.com",
		IPv6Network:      "fd12:198:18:252::/64",
		IPv6SubnetPrefix: 128,
		Routes:           []string{"default"},
	}}
	conf := RenderConf(o, nil)
	if !strings.Contains(conf, "[vhost:v.example.com]") {
		t.Fatal(conf)
	}
	if !strings.Contains(conf, "ipv6-subnet-prefix = 128") {
		t.Fatalf("%s", conf)
	}
	if !strings.Contains(conf, "route = ::/0") {
		t.Fatalf("%s", conf)
	}
}

func TestRenderConfNoIPv6OmitsPool(t *testing.T) {
	o := store.DefaultOCServ()
	conf := RenderConf(o, nil)
	if strings.Contains(conf, "ipv6-network") {
		t.Fatalf("unexpected ipv6:\n%s", conf)
	}
}
