package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderForwardDefaultDenyAndVPN(t *testing.T) {
	st := store.DefaultState()
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "9443"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `comment "qosnat2-forward-vpn-wg"`) {
		t.Fatal("missing forward wg accept")
	}
	if !strings.Contains(body, `comment "qosnat2-forward-warp-netns"`) {
		t.Fatal("missing forward warp netns accept")
	}
	if !strings.Contains(body, `comment "qosnat2-input-warp-netns"`) {
		t.Fatal("missing input warp netns accept")
	}
	if !strings.Contains(body, `comment "qosnat2-forward-vpn-ocserv"`) {
		t.Fatal("missing forward ocserv accept")
	}
	if !strings.Contains(body, `drop comment "qosnat2-forward-default-deny"`) {
		t.Fatal("missing forward default deny")
	}
	vpnIdx := strings.Index(body, `qosnat2-forward-vpn-wg`)
	warpFwdIdx := strings.Index(body, `qosnat2-forward-warp-netns`)
	denyIdx := strings.Index(body, `qosnat2-forward-default-deny`)
	if vpnIdx < 0 || warpFwdIdx < 0 || denyIdx < 0 || warpFwdIdx > denyIdx || vpnIdx > denyIdx {
		t.Fatalf("WARP/VPN forward must precede default deny:\n%s", body)
	}
	inputDenyIdx := strings.Index(body, `qosnat2-input-default-deny`)
	warpInIdx := strings.Index(body, `qosnat2-input-warp-netns`)
	if warpInIdx < 0 || inputDenyIdx < 0 || warpInIdx > inputDenyIdx {
		t.Fatalf("WARP input must precede default deny:\n%s", body)
	}
	// forward chain ends before input chain
	fwdEnd := strings.Index(body, "    }\n\n    chain input")
	if fwdEnd < 0 {
		t.Fatal("cannot locate forward chain end")
	}
	fwd := body[:fwdEnd]
	if strings.Count(fwd, `qosnat2-forward-default-deny`) != 1 {
		t.Fatal("forward default deny should appear once in forward chain")
	}
}
