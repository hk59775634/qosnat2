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
	if !strings.Contains(body, `iifname "ifb0" accept`) {
		t.Fatal("missing ifb0 accept")
	}
	if !strings.Contains(body, `drop comment "qosnat2-input-default-deny"`) {
		t.Fatal("missing input default deny")
	}
	lanIdx := strings.Index(body, `iifname "ens19" accept`)
	denyIdx := strings.Index(body, `qosnat2-input-default-deny`)
	if lanIdx < 0 || denyIdx < 0 || lanIdx > denyIdx {
		t.Fatalf("LAN accept must precede default deny in input chain:\n%s", body)
	}
}
