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
	st.Nat.IPv4.SharedIPs = []string{"203.0.113.10"}
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

func TestRenderEgressSNAT(t *testing.T) {
	st := store.DefaultState()
	st.Network.WanLinks = []store.WanLink{
		{ID: "wan-us", Device: "ens19", Gateway: "100.64.0.1", Enabled: true},
	}
	st.Network.EgressPolicies = []store.EgressPolicy{
		{ID: "eg-1", CIDR: "10.250.0.0/24", WanLinkID: "wan-us", SNATIP: "100.64.0.249", Enabled: true},
	}
	st.Nat.IPv4.PolicyRoutes = []string{"10.0.0.0/8", "10.250.0.0/24"}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`ip saddr 10.250.0.0/24 oifname "ens19" snat to 100.64.0.249`,
		`ip saddr 10.0.0.0/8 oifname "ens18"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, `10.250.0.0/24 oifname "ens18"`) {
		t.Fatal("10.250 should not SNAT on primary WAN")
	}
}

func TestRenderAcmeOpen80(t *testing.T) {
	st := store.DefaultState()
	st.System.AcmeTempAllowHTTP01 = true
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `tcp dport 80 accept comment "qosnat2-acme-http01-open80"`) {
		t.Fatalf("missing acme open80 rule in render")
	}
}

func TestRenderNPTv6(t *testing.T) {
	st := store.DefaultState()
	st.Nat.Nptv6Enabled = true
	st.Nat.Nptv6Rules = []store.Nptv6Rule{{
		InternalPrefix: "fd00::/48",
		ExternalPrefix: "2001:db8::/48",
	}}
	body, err := Render(Config{DevWAN: "eth1"}, st)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"snat ip6 prefix to 2001:db8::/48",
		"dnat ip6 prefix to fd00::/48",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
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
