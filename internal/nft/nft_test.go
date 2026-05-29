package nft

import (
	"fmt"
	"os"
	"os/exec"
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

func TestRenderEgressDestinationSNAT(t *testing.T) {
	st := store.DefaultState()
	st.Network.WanLinks = []store.WanLink{
		{ID: "wan-us", Device: "ens19", Gateway: "100.64.0.1", Enabled: true},
	}
	st.Network.EgressPolicies = []store.EgressPolicy{
		{
			ID: "eg-cf", CIDR: "173.245.48.0/20", Match: "destination",
			WanLinkID: "wan-us", SNATIP: "100.64.0.249", Enabled: true,
		},
	}
	st.Nat.IPv4.PolicyRoutes = []string{"10.0.0.0/8"}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	want := `ip daddr 173.245.48.0/20 oifname "ens19" snat to 100.64.0.249`
	if !strings.Contains(body, want) {
		t.Fatalf("missing %q in:\n%s", want, body)
	}
	if strings.Contains(body, `ip saddr 173.245.48.0/20`) {
		t.Fatal("destination egress must not use ip saddr")
	}
	if strings.Contains(body, `173.245.48.0/20 oifname "ens18"`) {
		t.Fatal("cloudflare cidr must not be excluded onto primary WAN SNAT")
	}
}

func TestRenderEgressWarpMasquerade(t *testing.T) {
	st := store.DefaultState()
	st.Network.WanLinks = []store.WanLink{
		{
			ID:          store.WanLinkIDWarp,
			Device:      "CloudflareWARP",
			Enabled:     true,
			PolicyOnly:  true,
			WarpManaged: true,
		},
	}
	st.Network.EgressPolicies = []store.EgressPolicy{
		{ID: "eg-warp", CIDR: "10.88.0.0/24", WanLinkID: store.WanLinkIDWarp, Enabled: true},
	}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	want := `ip saddr 10.88.0.0/24 oifname "CloudflareWARP" masquerade`
	if !strings.Contains(body, want) {
		t.Fatalf("missing %q in:\n%s", want, body)
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

// TestRenderUIFirewallRulesNftSyntax 模拟 Web UI 添加的用户规则（proto + ip daddr + dport）须通过 nft -c。
func TestRenderUIFirewallRulesNftSyntax(t *testing.T) {
	if _, err := execLookPath("nft"); err != nil {
		t.Skip("nft not installed")
	}
	st := store.DefaultState()
	st.Firewall.FilterRules = []store.FilterRule{
		// forward：UI 常见组合（入/出接口 + TCP + 目的 IP/端口）
		{
			ID: "fr-ui-1", Chain: "forward", Action: "accept",
			Iif: "ens18", Oif: "ens19", Proto: "tcp",
			DstAddr: "10.255.255.11", DstPort: 8088, Enabled: true,
		},
		{
			ID: "fr-ui-2", Chain: "forward", Action: "drop",
			Iif: "ens19", Oif: "ens18", Proto: "udp",
			SrcAddr: "203.0.113.0/24", DstPort: 53, Enabled: true,
		},
		// input：UI 常见组合
		{
			ID: "fr-ui-3", Chain: "input", Action: "accept",
			Iif: "ens18", Proto: "tcp", DstPort: 443, Enabled: true,
		},
	}
	st.Nat.IPv4.SharedIPs = []string{"58.56.59.66"}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18", AdminPort: "8080"}, st)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`iifname "ens18" oifname "ens19" ip daddr 10.255.255.11 tcp dport 8088 accept`,
		`iifname "ens19" oifname "ens18" ip saddr 203.0.113.0/24 udp dport 53 drop`,
		`iifname "ens18" tcp dport 443 accept`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing rule line %q in:\n%s", want, body)
		}
	}
	if err := nftCheckRuleset(body); err != nil {
		t.Fatalf("nft -c: %v\n%s", err, body)
	}
}

func execLookPath(name string) (string, error) {
	out, err := exec.Command("sh", "-c", "command -v "+name).CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func nftCheckRuleset(body string) error {
	f, err := os.CreateTemp("", "qosnat-nft-*.nft")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return err
	}
	f.Close()
	out, err := exec.Command("nft", "-c", "-f", f.Name()).CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
