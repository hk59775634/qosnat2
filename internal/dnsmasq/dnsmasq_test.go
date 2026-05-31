package dnsmasq

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderConfUpstreamDNS(t *testing.T) {
	dhcp := store.DefaultDHCP()
	dhcp.Enabled = true
	dhcp.DNSEnabled = true
	dhcp.Interface = "ens19"
	dhcp.UpstreamDNS = []string{"8.8.8.8", "1.1.1.1"}
	opts := ApplyOpts{ExceptWAN: "ens18", DevLAN: "ens19"}
	body := RenderConf(dhcp, opts)
	if !strings.Contains(body, "server=8.8.8.8\n") {
		t.Fatalf("missing upstream server:\n%s", body)
	}
	if !strings.Contains(body, "no-resolv\n") {
		t.Fatal("missing no-resolv")
	}
	if !strings.Contains(body, "port=53\n") {
		t.Fatal("missing port=53")
	}
	if !strings.Contains(body, "dhcp-option=option:dns-server,8.8.8.8,1.1.1.1") {
		t.Fatal("missing dhcp dns option")
	}
}

func TestRenderConfDHCPOnly(t *testing.T) {
	dhcp := store.DefaultDHCP()
	dhcp.Enabled = true
	dhcp.DNSEnabled = false
	dhcp.Interface = "ens19"
	opts := ApplyOpts{ExceptWAN: "ens18", DevLAN: "ens19"}
	body := RenderConf(dhcp, opts)
	if !strings.Contains(body, "port=0\n") {
		t.Fatalf("expected port=0 for dhcp-only:\n%s", body)
	}
	if strings.Contains(body, "server=8.8.8.8") {
		t.Fatal("should not have upstream without dns_enabled")
	}
	if !strings.Contains(body, "dhcp-range=") {
		t.Fatal("missing dhcp-range")
	}
}

func TestRenderConfDNSOnly(t *testing.T) {
	dhcp := store.DefaultDHCP()
	dhcp.Enabled = false
	dhcp.DNSEnabled = true
	dhcp.Interface = "ens19"
	dhcp.UpstreamDNS = []string{"1.1.1.1"}
	opts := ApplyOpts{ExceptWAN: "ens18", DevLAN: "ens19"}
	body := RenderConf(dhcp, opts)
	if strings.Contains(body, "dhcp-range=") {
		t.Fatalf("dhcp-only config should not have dhcp-range:\n%s", body)
	}
	if !strings.Contains(body, "port=53\n") {
		t.Fatal("missing port=53")
	}
	if !strings.Contains(body, "server=1.1.1.1\n") {
		t.Fatal("missing upstream")
	}
	if !strings.Contains(body, "interface=ens19\n") {
		t.Fatal("missing interface bind")
	}
}

func TestRenderConfUpstreamSkippedForDNS64Relay(t *testing.T) {
	dhcp := store.DefaultDHCP()
	dhcp.Enabled = true
	dhcp.DNSEnabled = true
	dhcp.Interface = "ens19"
	dhcp.UpstreamDNS = []string{"8.8.8.8"}
	nat := store.DefaultNat()
	nat.Nat64Enabled = true
	nat.DNS64 = store.DNS64Config{
		Mode:           store.DNS64ModeLocal,
		ServeToClients: true,
		UnboundListen:  "127.0.0.1:5353",
	}
	opts := ApplyOpts{ExceptWAN: "ens18", DevLAN: "ens19", Nat: nat}
	body := RenderConf(dhcp, opts)
	if strings.Contains(body, "server=8.8.8.8\n") {
		t.Fatalf("user upstream should be skipped when DNS64 relay active:\n%s", body)
	}
	if !strings.Contains(body, "server=127.0.0.1#5353") {
		t.Fatal("expected unbound upstream from DNS64 section")
	}
}

func TestRenderConfDisabled(t *testing.T) {
	dhcp := store.DefaultDHCP()
	opts := ApplyOpts{DevLAN: "ens19"}
	body := RenderConf(dhcp, opts)
	if !strings.Contains(body, "service disabled") {
		t.Fatalf("expected disabled comment:\n%s", body)
	}
}
