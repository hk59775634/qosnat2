package singbox

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestBuildConfigSocks5(t *testing.T) {
	p := store.ProxyEgress{
		ID: "pe-test", Name: "t", Type: "socks5",
		Server: "1.2.3.4", Port: 1080, Username: "u", Password: "p",
		TunIndex: 2,
	}
	cfg, err := BuildConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	inbounds, _ := cfg["inbounds"].([]any)
	if len(inbounds) != 1 {
		t.Fatalf("inbounds=%v", inbounds)
	}
	in := inbounds[0].(map[string]any)
	if in["interface_name"] != "qpe2" || in["auto_route"] != false {
		t.Fatalf("inbound=%v", in)
	}
	addrs, _ := in["address"].([]string)
	if len(addrs) != 1 || addrs[0] != "10.87.2.1/30" {
		t.Fatalf("address=%v", in["address"])
	}
	outs := cfg["outbounds"].([]any)
	ob := outs[0].(map[string]any)
	if ob["type"] != "socks" || ob["server"] != "1.2.3.4" {
		t.Fatalf("outbound=%v", ob)
	}
}

func TestBuildConfigHTTPS(t *testing.T) {
	p := store.ProxyEgress{
		ID: "pe-h", Type: "https", Server: "proxy.example.com", Port: 443, TunIndex: 0,
	}
	cfg, err := BuildConfig(p)
	if err != nil {
		t.Fatal(err)
	}
	ob := cfg["outbounds"].([]any)[0].(map[string]any)
	if ob["type"] != "http" {
		t.Fatalf("type=%v", ob["type"])
	}
	tls, ok := ob["tls"].(map[string]any)
	if !ok || tls["enabled"] != true {
		t.Fatalf("tls=%v", ob["tls"])
	}
}
