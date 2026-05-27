package store

import "testing"

func TestCollectDNS64AccessAllow(t *testing.T) {
	st := DefaultState()
	st.VPN.WireGuards[0].Enabled = true
	st.VPN.WireGuards[0].Address = "10.200.0.1/24"
	st.VPN.OCServ.IPv4Network = "10.250.0.0"
	st.VPN.OCServ.IPv4Netmask = "255.255.255.0"
	out := CollectDNS64AccessAllow(st)
	if len(out) < 2 {
		t.Fatalf("expected vpn pools, got %v", out)
	}
}

func TestEffectiveUnboundListenDirect(t *testing.T) {
	d := DNS64Config{ServeToClients: false}
	host, port, err := d.EffectiveUnboundListen("10.200.0.1")
	if err != nil || host != "10.200.0.1" || port != 53 {
		t.Fatalf("got %s:%d err=%v", host, port, err)
	}
}
