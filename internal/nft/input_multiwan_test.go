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

func TestRenderVXLANAutoWANInputAndForward(t *testing.T) {
	st := store.DefaultState()
	st.Network.VXLANTunnels = []store.VXLANTunnel{{
		ID: "vxlan-a", VNI: 100, Name: "vxlan100",
		Local: "203.0.113.10", Remote: "198.51.100.20", Port: 4789, Up: true,
	}}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "9443"}, st)
	if err != nil {
		t.Fatal(err)
	}
	wantInput := `iifname "ens18" ip saddr 198.51.100.20/32 udp dport 4789 accept`
	if !strings.Contains(body, wantInput) {
		t.Fatalf("missing vxlan wan udp accept %q in:\n%s", wantInput, body)
	}
	if !strings.Contains(body, `iifname "ens19" oifname "vxlan100" accept`) {
		t.Fatalf("missing LAN→vxlan forward in:\n%s", body)
	}
	if !strings.Contains(body, `iifname "vxlan*" accept comment "qosnat2-forward-vxlan"`) {
		t.Fatal("missing vxlan wildcard forward")
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
