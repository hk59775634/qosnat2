package nft

import "github.com/hk59775634/qosnat2/internal/store"

// VPNFirewall 入站链上随 VPN 服务启停放行的 WAN 端口。
type VPNFirewall struct {
	OCServEnabled bool
	OCServTCP     int
	OCServUDP     int
	WGEnabled     bool
	WGUDP         int
}

func VPNFirewallFromState(st store.State) VPNFirewall {
	o := st.VPN.OCServ
	wg := st.VPN.WireGuard
	tcp, udp := 443, 443
	if o.TCPPort > 0 {
		tcp = o.TCPPort
	}
	if o.UDPPort > 0 {
		udp = o.UDPPort
	} else {
		udp = tcp
	}
	wgPort := 51820
	if wg.ListenPort > 0 {
		wgPort = wg.ListenPort
	}
	return VPNFirewall{
		OCServEnabled: o.Enabled,
		OCServTCP:     tcp,
		OCServUDP:     udp,
		WGEnabled:     wg.Enabled,
		WGUDP:         wgPort,
	}
}
