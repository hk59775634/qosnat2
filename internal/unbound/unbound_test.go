package unbound

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderSingleForwardZone(t *testing.T) {
	nat := store.DefaultNat()
	nat.Nat64Enabled = true
	nat.DNS64.Mode = store.DNS64ModeLocal
	nat.DNS64.Forwarders = []string{"1.1.1.1", "8.8.8.8"}
	body, err := Render(nat, RenderOpts{GatewayIPv4: "10.0.0.1"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(body, "forward-zone:") != 1 {
		t.Fatalf("want one forward-zone, got:\n%s", body)
	}
	if !strings.Contains(body, "forward-addr: 1.1.1.1@53") || !strings.Contains(body, "forward-addr: 8.8.8.8@53") {
		t.Fatalf("missing forwarders:\n%s", body)
	}
}
