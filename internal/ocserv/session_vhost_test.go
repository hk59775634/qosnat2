package ocserv

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestResolveSessionVhostInfo(t *testing.T) {
	vhosts := []store.OCServVhost{
		{Enabled: true, Domain: "vpn.example.com", Comment: "总部 VPN"},
		{Enabled: true, Domain: "test.nip.io"},
		{Enabled: false, Domain: "disabled.example.com", Comment: "应忽略"},
	}

	cases := []struct {
		raw          string
		wantLabel    string
		wantHostname string
	}{
		{"", SessionVhostUnknown, ""},
		{"default", SessionVhostUnknown, ""},
		{"vpn.example.com", "总部 VPN", "vpn.example.com"},
		{"VPN.EXAMPLE.COM", "总部 VPN", "vpn.example.com"},
		{"test.nip.io", "test.nip.io", "test.nip.io"},
		{"other.example.com", SessionVhostUnknown, ""},
	}
	for _, c := range cases {
		label, host := ResolveSessionVhostInfo(c.raw, vhosts)
		if label != c.wantLabel || host != c.wantHostname {
			t.Fatalf("raw=%q got label=%q host=%q want %q / %q", c.raw, label, host, c.wantLabel, c.wantHostname)
		}
	}
}

func TestSessionRawVhost(t *testing.T) {
	s := map[string]any{"vhost": "vpn.example.com", "ID": 1}
	if got := SessionRawVhost(s); got != "vpn.example.com" {
		t.Fatalf("got %q", got)
	}
}
