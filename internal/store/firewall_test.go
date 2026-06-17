package store

import (
	"strings"
	"testing"
)

func TestFilterRuleNftLine(t *testing.T) {
	r := FilterRule{
		Chain: "forward", Action: "drop", Iif: "ens18", Oif: "ens19",
		SrcAddr: "203.0.113.0/24", DstPort: 443, Proto: "tcp", Enabled: true,
	}
	line := r.NftRuleLine()
	if line == "" {
		t.Fatal("empty line")
	}
	if !containsAll(line, "iifname", "drop", "tcp", "dport 443", "ip saddr") {
		t.Fatalf("unexpected: %s", line)
	}
	ipIdx := indexOf(line, "ip saddr")
	tcpIdx := indexOf(line, " tcp ")
	if ipIdx < 0 || tcpIdx < 0 || ipIdx > tcpIdx {
		t.Fatalf("ip match must precede tcp: %s", line)
	}
}

func TestFilterRuleNftLineIcmp(t *testing.T) {
	r := FilterRule{
		ID: "fr-icmp", Chain: "forward", Action: "accept",
		Iif: "ens19", Oif: "ens18", Proto: "icmp", Enabled: true,
	}
	line := r.NftRuleLine()
	if !containsAll(line, `iifname "ens19"`, `oifname "ens18"`, "meta l4proto icmp", " accept") {
		t.Fatalf("unexpected icmp line: %s", line)
	}
	if strings.Contains(line, `oifname "ens18" icmp `) {
		t.Fatalf("bare icmp must not appear: %s", line)
	}
}

func TestFilterRuleNftLineIcmpv6(t *testing.T) {
	r := FilterRule{
		Chain: "input", Action: "accept", Iif: "ens18",
		Proto: "icmpv6", IPVersion: "ipv6", Enabled: true,
	}
	line := r.NftRuleLine()
	if !contains(line, "meta l4proto ipv6-icmp") {
		t.Fatalf("unexpected icmpv6 line: %s", line)
	}
}

func TestFilterRuleNftLineComment(t *testing.T) {
	r := FilterRule{
		ID: "fr-1", Chain: "input", Action: "accept", Iif: "ens18", Proto: "tcp", DstPort: 443,
		Comment: `note "admin"`, Enabled: true,
	}
	line := r.NftRuleLine()
	if !contains(line, `comment "note \"admin\" qosnat2:rid:fr-1"`) {
		t.Fatalf("expected escaped comment with id marker in: %s", line)
	}
	acceptIdx := indexOf(line, " accept")
	commentIdx := indexOf(line, " comment ")
	if acceptIdx < 0 || commentIdx < acceptIdx {
		t.Fatalf("comment must follow action: %s", line)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !contains(s, p) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
