package store

import "testing"

func TestCollectSessionLimitCIDRs(t *testing.T) {
	st := DefaultState()
	st.Shaper.PolicyCIDR = "10.254.0.0/15"
	st.Nat.IPv4.PolicyRoutes = []string{"10.0.0.0/8"}
	st.DHCP.Enabled = true
	st.DHCP.Router = "192.168.1.1"
	st.DHCP.Netmask = "255.255.255.0"
	st.VPN.OCServ.Enabled = true
	st.VPN.OCServ.IPv4Network = "10.250.0.0"
	st.VPN.OCServ.IPv4Netmask = "255.255.0.0"
	st.VPN.WireGuards = []WireGuardInstance{{
		ID:   "wg1",
		Mode: WGModeServer,
		WireGuardState: WireGuardState{
			Enabled:   true,
			Address:   "10.8.0.1/24",
			ListenPort: 51820,
			Peers: []WGPeer{{
				Name:       "c1",
				AllowedIPs: []string{"10.8.0.2/32", "0.0.0.0/0"},
			}},
		},
	}}
	got := CollectSessionLimitCIDRs(st)
	want := map[string]bool{
		"10.254.0.0/15":  true,
		"10.0.0.0/8":     true,
		"192.168.1.0/24": true,
		"10.250.0.0/16":  true,
		"10.8.0.0/24":    true,
		"10.8.0.2/32":    true,
	}
	if len(got) < len(want) {
		t.Fatalf("got %v want at least %v", got, want)
	}
	for _, c := range got {
		if c == "0.0.0.0/0" {
			t.Fatal("must not include default route")
		}
		delete(want, c)
	}
	if len(want) > 0 {
		t.Fatalf("missing cidrs %v in %v", want, got)
	}
}

func TestNormalizeMaxSessionsPerIP(t *testing.T) {
	if _, err := NormalizeMaxSessionsPerIP(-1); err == nil {
		t.Fatal("expected error")
	}
	n, err := NormalizeMaxSessionsPerIP(500)
	if err != nil || n != 500 {
		t.Fatalf("got %d %v", n, err)
	}
}
