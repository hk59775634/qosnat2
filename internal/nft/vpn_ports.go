package nft

import (
	"sort"

	"github.com/hk59775634/qosnat2/internal/store"
)

// VPNFirewall 入站链上随 VPN 服务启停放行的 WAN 端口。
type VPNFirewall struct {
	OCServEnabled bool
	OCServTCP     int
	OCServUDP     int
	// WGPorts 为已启用 WireGuard **服务端**实例的 UDP 监听端口（多实例各端口一条 accept）。
	WGPorts []int
}

func VPNFirewallFromState(st store.State) VPNFirewall {
	o := st.VPN.OCServ
	tcp, udp := 443, 443
	if o.TCPPort > 0 {
		tcp = o.TCPPort
	}
	if o.UDPPort > 0 {
		udp = o.UDPPort
	} else {
		udp = tcp
	}
	seen := map[int]struct{}{}
	var wgPorts []int
	for _, inst := range st.VPN.WireGuards {
		if inst.Mode != store.WGModeServer || !inst.Enabled {
			continue
		}
		p := inst.ListenPort
		if p <= 0 {
			p = 51820
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		wgPorts = append(wgPorts, p)
	}
	sort.Ints(wgPorts)
	return VPNFirewall{
		OCServEnabled: o.Enabled,
		OCServTCP:     tcp,
		OCServUDP:     udp,
		WGPorts:       wgPorts,
	}
}
