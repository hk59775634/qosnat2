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
	if !strings.Contains(body, `iifname "wg*" accept`) {
		t.Fatal("missing vpn wg accept")
	}
	if !strings.Contains(body, `iifname "vpns*" accept`) {
		t.Fatal("missing vpn ocserv accept")
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

func TestRenderUserWANInputDropAfterAdminAccept(t *testing.T) {
	st := store.DefaultState()
	st.Firewall.FilterRules = []store.FilterRule{{
		ID: "fr-wan-drop", Chain: "input", Action: "drop", Iif: "ens18", Enabled: true,
	}}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "9443"}, st)
	if err != nil {
		t.Fatal(err)
	}
	input := inputChainBody(body)
	admin := strings.Index(input, `iifname "ens18" tcp dport 9443 accept`)
	drop := strings.Index(input, `iifname "ens18" drop`)
	if admin < 0 || drop < 0 {
		t.Fatalf("missing admin or drop in input chain:\n%s", input)
	}
	if admin > drop {
		t.Fatalf("admin accept must precede WAN drop rules:\n%s", input)
	}
}

func inputChainBody(body string) string {
	const marker = "chain input {"
	start := strings.Index(body, marker)
	if start < 0 {
		return body
	}
	depth := 0
	for i := start; i < len(body); i++ {
		switch body[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return body[start : i+1]
			}
		}
	}
	return body[start:]
}
