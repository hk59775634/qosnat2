package nft

import (
	"github.com/hk59775634/qosnat2/internal/store"
)

// AutoInputFromState 汇总 VPN 与 SNMP 等待生成的 WAN input 自动规则参数。
func AutoInputFromState(st store.State) store.AutoInputVPN {
	vp := VPNFirewallFromState(st)
	ai := store.AutoInputVPN{
		OCServEnabled: vp.OCServEnabled,
		OCServTCP:     vp.OCServTCP,
		OCServUDP:     vp.OCServUDP,
		WGPorts:       vp.WGPorts,
	}
	cfg := st.SNMP
	if err := store.NormalizeSNMP(&cfg); err != nil {
		return ai
	}
	if cfg.Enabled {
		ai.SNMPEnabled = true
		ai.SNMPPort = cfg.Port
		if ai.SNMPPort <= 0 {
			ai.SNMPPort = 161
		}
		ai.SNMPAllowedNetworks = append([]string(nil), cfg.AllowedNetworks...)
	}
	return ai
}
