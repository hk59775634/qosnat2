package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderMultiWANAutoInput(t *testing.T) {
	st := store.DefaultState()
	st.Network.WanLinks = []store.WanLink{
		{ID: "wan2", Device: "ens20", Enabled: true},
	}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "9443"}, st)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`iifname "ens18" tcp dport 9443 accept`,
		`iifname "ens20" tcp dport 9443 accept`,
		`iifname "ens18" drop`,
		`iifname "ens20" drop`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestRenderMultiWANForwardLAN(t *testing.T) {
	st := store.DefaultState()
	st.Network.WanLinks = []store.WanLink{
		{ID: "wan2", Device: "ens20", Enabled: true},
	}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	want := `iifname "ens19" oifname "ens20" accept`
	if !strings.Contains(body, want) {
		t.Fatalf("missing forward %q in:\n%s", want, body)
	}
}
