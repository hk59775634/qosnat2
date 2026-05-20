package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func writeWanForwardRules(b *strings.Builder, cfg Config, forwards []store.WanPortForward) {
	for _, f := range forwards {
		iface := strings.TrimSpace(f.Interface)
		if iface == "" {
			iface = cfg.DevWAN
		}
		for _, proto := range store.ForwardProtos(f.Proto) {
			b.WriteString(buildDNATLine(iface, f, proto))
		}
	}
}

func buildDNATLine(iface string, f store.WanPortForward, proto string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf(`iifname "%s"`, iface))
	if f.IPVersion == "ipv6" {
		parts = append(parts, "meta nfproto ipv6")
		if !store.IsAnyCIDR(f.SrcAddr) {
			parts = append(parts, "ip6 saddr "+f.SrcAddr)
		}
		if d := strings.TrimSpace(f.DstAddr); d != "" && !store.IsAnyCIDR(d) {
			parts = append(parts, "ip6 daddr "+d)
		}
		parts = append(parts, proto, fmt.Sprintf("dport %d", f.DstPort))
		parts = append(parts, "dnat ip6 to "+formatDNATTarget6(f.RedirectIP, f.RedirectPort))
	} else {
		parts = append(parts, "meta nfproto ipv4")
		if !store.IsAnyCIDR(f.SrcAddr) {
			parts = append(parts, "ip saddr "+f.SrcAddr)
		}
		if d := strings.TrimSpace(f.DstAddr); d != "" && !store.IsAnyCIDR(d) {
			parts = append(parts, "ip daddr "+d)
		}
		parts = append(parts, proto, fmt.Sprintf("dport %d", f.DstPort))
		parts = append(parts, "dnat to "+formatDNATTarget4(f.RedirectIP, f.RedirectPort))
	}
	return "        " + strings.Join(parts, " ") + "\n"
}

func formatDNATTarget4(ip string, port int) string {
	return fmt.Sprintf("%s:%d", ip, port)
}

func formatDNATTarget6(ip string, port int) string {
	if strings.Contains(ip, ":") {
		return fmt.Sprintf("[%s]:%d", ip, port)
	}
	return fmt.Sprintf("%s:%d", ip, port)
}
