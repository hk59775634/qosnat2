package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderAutoWANInputRules(t *testing.T) {
	st := store.DefaultState()
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "9443"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `iifname "ens18" tcp dport 9443 accept`) {
		t.Fatalf("missing admin auto rule in:\n%s", body)
	}
	if !strings.Contains(body, `iifname "ens18" drop`) {
		t.Fatal("missing wan drop")
	}
}
