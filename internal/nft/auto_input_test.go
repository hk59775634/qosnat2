package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderAutoWANInputSNMP(t *testing.T) {
	st := store.DefaultState()
	st.SNMP = store.SNMPState{
		Enabled:             true,
		Port:                161,
		ListenLocalhostOnly: false,
		ROCommunity:         "public",
		AllowedNetworks:     []string{"203.0.113.0/24"},
	}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "8080"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `iifname "ens18" ip saddr 203.0.113.0/24 udp dport 161 accept`) {
		t.Fatalf("missing snmp wan rule in:\n%s", body)
	}
}

func TestRenderAutoWANInputSNMPLocalhostOnly(t *testing.T) {
	st := store.DefaultState()
	st.SNMP = store.SNMPState{
		Enabled:             true,
		Port:                161,
		ListenLocalhostOnly: true,
		ROCommunity:         "public",
	}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "8080"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "auto-input-snmp") {
		t.Fatalf("localhost-only snmp must not open wan:\n%s", body)
	}
}
