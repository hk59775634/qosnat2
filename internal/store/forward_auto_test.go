package store

import "testing"

func TestBuildAutoForwardFilterRules(t *testing.T) {
	fwd := []WanPortForward{{
		ID: "fwd-abc", Interface: "eth0", IPVersion: "ipv4", Proto: "tcp_udp",
		SrcAddr: "203.0.113.0/24", DstPort: 443, RedirectIP: "192.168.1.10", RedirectPort: 8443,
		Comment: "web",
	}}
	rules := BuildAutoForwardFilterRules(fwd, "br-lan")
	if len(rules) != 2 {
		t.Fatalf("want 2 rules, got %d", len(rules))
	}
	if rules[0].ID != "auto-fwd-fwd-abc-tcp" {
		t.Fatalf("id: %s", rules[0].ID)
	}
	if rules[0].Chain != "forward" || rules[0].Oif != "br-lan" || rules[0].Iif != "eth0" {
		t.Fatalf("unexpected rule: %+v", rules[0])
	}
	if rules[0].DstAddr != "192.168.1.10" || rules[0].DstPort != 8443 {
		t.Fatalf("dst: %+v", rules[0])
	}
	if rules[0].SrcAddr != "203.0.113.0/24" {
		t.Fatalf("src: %s", rules[0].SrcAddr)
	}
	if !rules[0].System || !IsAutoForwardRule(rules[0]) {
		t.Fatal("should be auto forward rule")
	}
}

func TestBuildAutoHairpinForwardFilterRules(t *testing.T) {
	fwd := []WanPortForward{{
		ID: "fwd-abc", Interface: "eth0", IPVersion: "ipv4", Proto: "tcp",
		DstPort: 443, RedirectIP: "192.168.1.10", RedirectPort: 8443,
	}}
	rules := BuildAutoHairpinForwardFilterRules(fwd, "br-lan", nil)
	if len(rules) != 1 {
		t.Fatalf("want 1 rule, got %d", len(rules))
	}
	if rules[0].ID != "auto-fwd-hairpin-fwd-abc-tcp" {
		t.Fatalf("id: %s", rules[0].ID)
	}
	if rules[0].Iif != "br-lan" || rules[0].Oif != "br-lan" {
		t.Fatalf("unexpected: %+v", rules[0])
	}
}

func TestBuildAutoHairpinInputRules(t *testing.T) {
	resolver := HairpinAddrResolver{
		PrimaryIPv4: func(dev string) (string, error) {
			if dev == "eth0" {
				return "203.0.113.10", nil
			}
			return "", nil
		},
	}
	fwd := []WanPortForward{{
		ID: "fwd-1", Interface: "eth0", IPVersion: "ipv4", Proto: "tcp",
		DstPort: 443, RedirectIP: "192.168.1.10", RedirectPort: 443,
	}}
	rules := BuildAutoHairpinInputRules([]string{"eth0"}, "8443", AutoInputVPN{}, fwd, "br-lan", resolver)
	var admin, ssh, portFwd bool
	for _, r := range rules {
		if r.ID == "auto-input-hairpin-admin-eth0" && r.DstAddr == "203.0.113.10" && r.DstPort == 8443 {
			admin = true
		}
		if r.ID == "auto-input-hairpin-ssh-eth0" && r.DstAddr == "203.0.113.10" && r.DstPort == DefaultSSHPort {
			ssh = true
		}
		if r.ID == "auto-input-hairpin-fwd-fwd-1-tcp" && r.DstAddr == "203.0.113.10" && r.DstPort == 443 {
			portFwd = true
		}
	}
	if !admin || !ssh || !portFwd {
		t.Fatalf("missing hairpin input rules: %+v", rules)
	}
}

func TestSyncAutoFilterRulesRemovesStaleForwardRules(t *testing.T) {
	stale := FilterRule{ID: "auto-fwd-old-udp", Chain: "forward", Action: "accept", System: true}
	user := FilterRule{ID: "fr-1", Chain: "forward", Action: "drop", Enabled: true}
	fwd := []WanPortForward{{
		ID: "fwd-new", Interface: "wan0", Proto: "tcp", DstPort: 80,
		RedirectIP: "10.0.0.2", RedirectPort: 8080,
	}}
	merged, changed := SyncAutoFilterRules([]FilterRule{user, stale}, []string{"wan0"}, "8080", AutoInputVPN{}, fwd, LVSState{}, "lan0", "wan0", HairpinAddrResolver{})
	if !changed {
		t.Fatal("expected change")
	}
	for _, r := range merged {
		if r.ID == "auto-fwd-old-udp" {
			t.Fatal("stale forward auto rule should be removed")
		}
	}
	found := false
	for _, r := range merged {
		if r.ID == "auto-fwd-fwd-new-tcp" {
			found = true
		}
	}
	if !found {
		t.Fatal("missing new auto forward rule")
	}
}
