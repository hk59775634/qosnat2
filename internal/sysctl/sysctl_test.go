package sysctl

import "testing"

func TestCatalogLFNKeys(t *testing.T) {
	need := []string{
		"net.ipv4.tcp_congestion_control",
		"net.ipv4.tcp_rmem",
		"net.ipv4.tcp_wmem",
		"net.core.default_qdisc",
		"net.ipv4.tcp_mtu_probing",
	}
	keys := map[string]Entry{}
	for _, e := range Catalog {
		keys[e.Key] = e
	}
	for _, k := range need {
		if _, ok := keys[k]; !ok {
			t.Fatalf("catalog missing %s", k)
		}
		if Defaults[k] == "" {
			t.Fatalf("Defaults missing %s", k)
		}
	}
	if Defaults["net.ipv4.tcp_congestion_control"] != "cubic" {
		t.Fatal(Defaults["net.ipv4.tcp_congestion_control"])
	}
}

func TestValidateValue(t *testing.T) {
	if err := ValidateValue("134217728"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateValue("1\n2"); err == nil {
		t.Fatal("expected newline rejection")
	}
	if err := ValidateValue("a=b"); err == nil {
		t.Fatal("expected equals rejection")
	}
}
