package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
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

// writeWanForwardHairpinRules 内网访问公网 IP:端口时的回流 DNAT（NAT reflection）。
func writeWanForwardHairpinRules(b *strings.Builder, cfg Config, forwards []store.WanPortForward) {
	if cfg.DevLAN == "" {
		return
	}
	primary4 := netif.PrimaryIPv4
	primary6 := netif.PrimaryIPv6
	for _, f := range forwards {
		iface := strings.TrimSpace(f.Interface)
		if iface == "" {
			iface = cfg.DevWAN
		}
		wanIP := store.HairpinMatchAddr(f, iface, primary4, primary6)
		if wanIP == "" {
			continue
		}
		for _, proto := range store.ForwardProtos(f.Proto) {
			if line := buildHairpinPreroutingLine(cfg.DevLAN, wanIP, f, proto); line != "" {
				b.WriteString(line)
			}
		}
	}
}

func writeWanForwardHairpinSNAT(b *strings.Builder, cfg Config, forwards []store.WanPortForward) {
	if cfg.DevLAN == "" {
		return
	}
	seen := map[string]struct{}{}
	for _, f := range forwards {
		rip := strings.TrimSpace(f.RedirectIP)
		if rip == "" {
			continue
		}
		if _, ok := seen[rip]; ok {
			continue
		}
		seen[rip] = struct{}{}
		if f.IPVersion == "ipv6" {
			b.WriteString(fmt.Sprintf(
				"        ip6 saddr %s oifname \"%s\" masquerade comment \"qosnat2-fwd-hairpin-%s\"\n",
				rip, cfg.DevLAN, f.ID,
			))
		} else {
			b.WriteString(fmt.Sprintf(
				"        ip saddr %s oifname \"%s\" masquerade comment \"qosnat2-fwd-hairpin-%s\"\n",
				rip, cfg.DevLAN, f.ID,
			))
		}
	}
}

func buildHairpinPreroutingLine(devLAN, wanIP string, f store.WanPortForward, proto string) string {
	var parts []string
	parts = append(parts, fmt.Sprintf(`iifname "%s"`, devLAN))
	if f.IPVersion == "ipv6" {
		parts = append(parts, "meta nfproto ipv6", "ip6 daddr "+wanIP)
		if !store.IsAnyCIDR(f.SrcAddr) {
			parts = append(parts, "ip6 saddr "+f.SrcAddr)
		}
		parts = append(parts, proto, fmt.Sprintf("dport %d", f.DstPort))
		parts = append(parts, "dnat ip6 to "+formatDNATTarget6(f.RedirectIP, f.RedirectPort))
	} else {
		parts = append(parts, "meta nfproto ipv4", "ip daddr "+wanIP)
		if !store.IsAnyCIDR(f.SrcAddr) {
			parts = append(parts, "ip saddr "+f.SrcAddr)
		}
		parts = append(parts, proto, fmt.Sprintf("dport %d", f.DstPort))
		parts = append(parts, "dnat to "+formatDNATTarget4(f.RedirectIP, f.RedirectPort))
	}
	return "        " + strings.Join(parts, " ") + "\n"
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
