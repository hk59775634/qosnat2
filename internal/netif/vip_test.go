package netif

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestMergeVirtualIPsIntoIfaces(t *testing.T) {
	ifaces := []store.IfaceConfig{
		{Device: "ens18", IPv4: []string{"198.51.100.1/24"}, Up: true},
	}
	vips := []store.VirtualIP{
		{ID: "vip-1", Type: store.VirtualIPTypeIPAlias, Interface: "ens18", Address: "203.0.113.10/32", Enabled: true},
		{ID: "vip-2", Type: store.VirtualIPTypeIPAlias, Interface: "ens19", Address: "203.0.113.20/32", Enabled: true},
		{ID: "vip-3", Type: store.VirtualIPTypeIPAlias, Interface: "ens18", Address: "203.0.113.11/32", Enabled: false},
	}
	out := MergeVirtualIPsIntoIfaces(ifaces, vips)
	if len(out) != 1 || len(out[0].IPv4) != 2 {
		t.Fatalf("%+v", out)
	}
	joined := strings.Join(out[0].IPv4, ",")
	if !strings.Contains(joined, "203.0.113.10/32") || strings.Contains(joined, "203.0.113.11") {
		t.Fatalf("merged=%s", joined)
	}
	// original unchanged
	if len(ifaces[0].IPv4) != 1 {
		t.Fatal("original mutated")
	}
}

func TestRenderNetplanMergesVIP(t *testing.T) {
	body, _, err := RenderNetplan(store.NetworkState{
		Ifaces: []store.IfaceConfig{
			{Device: "ens18", IPv4: []string{"198.51.100.1/24"}, Up: true},
		},
		VirtualIPs: []store.VirtualIP{
			{Type: store.VirtualIPTypeIPAlias, Interface: "ens18", Address: "203.0.113.10", Enabled: true},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "203.0.113.10/32") {
		t.Fatalf("missing vip in yaml: %s", s)
	}
}
