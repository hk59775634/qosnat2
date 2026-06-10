package frr

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderDynamicBGP(t *testing.T) {
	cfg := store.DynamicRoutingState{
		BGP: store.BGPConfig{
			Enabled:  true,
			ASN:      65001,
			RouterID: "1.1.1.1",
			Neighbors: []store.BGPNeighbor{{
				Address:   "10.0.0.2",
				RemoteASN: 65002,
				Enabled:   true,
			}},
			Networks:              []string{"10.0.0.0/24"},
			RedistributeConnected: true,
		},
	}
	out := RenderDynamic(cfg)
	for _, want := range []string{
		"router bgp 65001",
		"bgp router-id 1.1.1.1",
		"neighbor 10.0.0.2 remote-as 65002",
		"network 10.0.0.0/24",
		"redistribute connected",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in:\n%s", want, out)
		}
	}
}

func TestRenderDynamicApplyScriptClears(t *testing.T) {
	out := RenderDynamicApplyScript(store.DynamicRoutingState{})
	if !strings.Contains(out, "no router bgp") || !strings.Contains(out, "no router ospf") {
		t.Fatalf("expected cleanup commands: %s", out)
	}
}
