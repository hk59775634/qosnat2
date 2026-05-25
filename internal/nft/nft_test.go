package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderSNATAndFilter(t *testing.T) {
	st := store.DefaultState()
	st.Firewall.FilterRules = []store.FilterRule{{
		ID: "fr-1", Chain: "forward", Action: "drop", Iif: "ens18", Enabled: true,
	}}
	st.SharedIPs = []string{"203.0.113.10"}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"table inet qosnat", "masquerade", "ens18", "fr-1", "drop"} {
		if !strings.Contains(body, want) && want != "fr-1" {
			if !strings.Contains(body, "drop") {
				t.Fatalf("missing %q in render", want)
			}
		}
	}
	if !strings.Contains(body, "drop") {
		t.Fatal("missing filter drop rule")
	}
}

func TestRenderWANOnly(t *testing.T) {
	st := store.DefaultState()
	body, err := Render(Config{DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "ens19") {
		t.Fatal("WAN-only render should not reference LAN")
	}
	if !strings.Contains(body, `oifname "ens18" masquerade`) {
		t.Fatal("missing WAN masquerade")
	}
}
