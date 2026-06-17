package nft

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderFilterRuleIcmpNftCheck(t *testing.T) {
	st := store.DefaultState()
	st.Firewall.FilterRules = []store.FilterRule{{
		ID: "fr-icmp", Chain: "forward", Action: "accept",
		Iif: "ens19", Oif: "ens18", Proto: "icmp", Enabled: true,
	}}
	body, err := Render(Config{DevLAN: "ens19", DevWAN: "ens18"}, st)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, `meta l4proto icmp`) {
		t.Fatalf("missing icmp l4proto in:\n%s", body)
	}
	if err := nftCheckRuleset(body); err != nil {
		if strings.Contains(err.Error(), "skip:") {
			t.Skip(strings.TrimPrefix(err.Error(), "skip: "))
		}
		t.Fatalf("nft check: %v\n%s", err, body)
	}
}
