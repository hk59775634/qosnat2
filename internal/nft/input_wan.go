package nft

import (
	"fmt"
	"strings"
)

// writeWANInputPolicy 在 input 链末尾：WAN 放行管理端口与已启用 VPN 监听端口，其余 WAN 入站丢弃。
func writeWANInputPolicy(b *strings.Builder, cfg Config) {
	if cfg.DevWAN == "" {
		return
	}
	wan := cfg.DevWAN
	adminPort := strings.TrimSpace(cfg.AdminPort)
	if adminPort == "" {
		adminPort = "8080"
	}
	b.WriteString(fmt.Sprintf("        iifname \"%s\" tcp dport %s accept comment \"qosnat2-admin\"\n", wan, adminPort))
	vp := cfg.VPN
	if vp.OCServEnabled && vp.OCServTCP > 0 {
		b.WriteString(fmt.Sprintf("        iifname \"%s\" tcp dport %d accept comment \"qosnat2-ocserv-tcp\"\n", wan, vp.OCServTCP))
	}
	if vp.OCServEnabled && vp.OCServUDP > 0 {
		b.WriteString(fmt.Sprintf("        iifname \"%s\" udp dport %d accept comment \"qosnat2-ocserv-udp\"\n", wan, vp.OCServUDP))
	}
	if vp.WGEnabled && vp.WGUDP > 0 {
		b.WriteString(fmt.Sprintf("        iifname \"%s\" udp dport %d accept comment \"qosnat2-wireguard\"\n", wan, vp.WGUDP))
	}
	b.WriteString(fmt.Sprintf("        iifname \"%s\" drop comment \"qosnat2-wan-default-drop\"\n", wan))
}
