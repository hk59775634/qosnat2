package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderWanForwardHairpin(t *testing.T) {
	st := store.DefaultState()
	st.Firewall.WanPortForwards = []store.WanPortForward{{
		ID: "fwd-1", Interface: "ens18", IPVersion: "ipv4", Proto: "tcp",
		DstAddr: "203.0.113.10", DstPort: 443, RedirectIP: "192.168.1.10", RedirectPort: 443,
	}}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	wantPreroute := `iifname "ens19" meta nfproto ipv4 ip daddr 203.0.113.10 tcp dport 443 dnat to 192.168.1.10:443`
	if !strings.Contains(body, wantPreroute) {
		t.Fatalf("missing hairpin prerouting:\n%s", body)
	}
	wantSNAT := `ip saddr 192.168.1.10 oifname "ens19" masquerade`
	if !strings.Contains(body, wantSNAT) {
		t.Fatalf("missing hairpin snat:\n%s", body)
	}
	if !strings.Contains(body, `iifname "ens18" oifname "ens19" ip daddr 192.168.1.10 tcp dport 443 accept`) {
		t.Fatalf("missing auto forward filter in render:\n%s", body)
	}
	if !strings.Contains(body, `iifname "ens19" oifname "ens19" ip daddr 192.168.1.10 tcp dport 443 accept`) {
		t.Fatalf("missing hairpin forward filter in render:\n%s", body)
	}
}

func TestRenderWanForwardHairpinSkipsGatewayLocal(t *testing.T) {
	st := store.DefaultState()
	st.Firewall.WanPortForwards = []store.WanPortForward{{
		ID: "fwd-local", Interface: "ens18", IPVersion: "ipv4", Proto: "tcp",
		DstAddr: "203.0.113.10", DstPort: 443, RedirectIP: "127.0.0.1", RedirectPort: 443,
	}}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, `iifname "ens19" meta nfproto ipv4 ip daddr 203.0.113.10`) {
		t.Fatalf("should not hairpin DNAT to gateway-local redirect:\n%s", body)
	}
	if !strings.Contains(body, `iifname "ens19" ip daddr 203.0.113.10 tcp dport 443 accept`) {
		t.Fatalf("missing hairpin input for gateway-local forward:\n%s", body)
	}
	if strings.Contains(body, `iifname "ens19" oifname "ens19" ip daddr 127.0.0.1`) {
		t.Fatalf("should not hairpin forward for gateway-local redirect:\n%s", body)
	}
}
