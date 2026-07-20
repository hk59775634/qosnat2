package store

import (
	"fmt"
	"sort"
	"strings"
)

// Auto rule IDs (reserved prefix auto-).
const (
	autoIDInputAdmin        = "auto-input-admin"
	autoIDInputSSH          = "auto-input-ssh"
	autoIDInputHairpinAdmin = "auto-input-hairpin-admin"
	autoIDInputHairpinSSH   = "auto-input-hairpin-ssh"
	autoIDInputHairpinFwd   = "auto-input-hairpin-fwd"
	autoIDInputOcservTCP    = "auto-input-ocserv-tcp"
	autoIDInputOcservUDP    = "auto-input-ocserv-udp"
	autoIDInputLVSPrefix    = "auto-input-lvs"
	autoIDInputSNMPPrefix   = "auto-input-snmp"
	autoIDInputWanDrop      = "auto-input-wan-drop"
)

// DefaultSSHPort is the host SSH port always opened on WAN input by default
// so operators are not locked out after firewall apply (before recording Web UI credentials).
const DefaultSSHPort = 22

// AutoInputVPN mirrors nft.VPNFirewall without importing nft.
type AutoInputVPN struct {
	OCServEnabled bool
	OCServTCP     int
	OCServUDP     int
	WGPorts       []int
	// SNMP：启用且非仅本机监听时，在 WAN 放行 UDP 端口（源地址受 allowed_networks 约束）。
	SNMPEnabled         bool
	SNMPPort            int
	SNMPAllowedNetworks []string
	// LVS VIP 入站放行（DstAddr=VIP）。
	LVSEndpoints []AutoInputLVSEndpoint
}

// AutoInputLVSEndpoint WAN input 链需放行的 LVS 虚拟服务（单协议）。
type AutoInputLVSEndpoint struct {
	VSID  string
	VIP   string
	Port  int
	Proto string
}

// IsAutoManagedRule 平台自动同步的受管规则（不可编辑/删除/排序）。
func IsAutoManagedRule(r FilterRule) bool {
	if r.System {
		return true
	}
	id := strings.TrimSpace(r.ID)
	return strings.HasPrefix(id, "auto-")
}

// CollectWanInputDevices 返回需生成 WAN 入站自动规则的网卡（主 WAN + 已启用 WanLink，排除 LAN）。
func CollectWanInputDevices(devWAN, devLAN string, st State) []string {
	devLAN = strings.TrimSpace(devLAN)
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == devLAN {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	add(devWAN)
	for _, w := range st.Network.WanLinks {
		if !w.Enabled {
			continue
		}
		// WARP/ProxyEgress 虚拟口不是真实 WAN 上联；入站默认丢弃会误伤 TUN/veth 回程。
		if IsManagedWanLink(w) {
			continue
		}
		add(w.Device)
	}
	sort.Strings(out)
	return out
}

// CollectWanForwardDevices 返回 forward 链需处理的 WAN 口（主 WAN + 已启用 WanLink + 已解析 egress，排除 LAN）。
func CollectWanForwardDevices(devWAN, devLAN string, st State, egressDevices []string) []string {
	devLAN = strings.TrimSpace(devLAN)
	seen := map[string]struct{}{}
	var out []string
	add := func(name string) {
		name = strings.TrimSpace(name)
		if name == "" || name == devLAN {
			return
		}
		if _, ok := seen[name]; ok {
			return
		}
		seen[name] = struct{}{}
		out = append(out, name)
	}
	for _, d := range CollectWanInputDevices(devWAN, devLAN, st) {
		add(d)
	}
	for _, d := range egressDevices {
		add(d)
	}
	sort.Strings(out)
	return out
}

// BuildAutoInputRules 为每个 WAN 口生成 input 链自动规则（放行项 → 该口默认丢弃）。
// 默认放行：管理口、SSH/22、已启用 VPN/SNMP/LVS 端口。
func BuildAutoInputRules(wanDevs []string, adminPort string, vpn AutoInputVPN) []FilterRule {
	if len(wanDevs) == 0 {
		return nil
	}
	port := strings.TrimSpace(adminPort)
	if port == "" {
		port = "8080"
	}
	adminP := parsePortInt(port)
	var out []FilterRule
	for _, wan := range wanDevs {
		wan = strings.TrimSpace(wan)
		if wan == "" {
			continue
		}
		sfx := wan
		out = append(out, FilterRule{
			ID:      autoIDInputAdmin + "-" + sfx,
			Chain:   "input",
			Action:  "accept",
			Iif:     wan,
			Proto:   "tcp",
			DstPort: adminP,
			Comment: fmt.Sprintf("qosnat2 管理端口 %s（自动）", wan),
			Enabled: true,
			System:  true,
		})
		// Always allow SSH on WAN so default deploy cannot lock out shell access.
		// Skip when admin port is already 22 (same tcp dport would duplicate).
		if adminP != DefaultSSHPort {
			out = append(out, FilterRule{
				ID:      autoIDInputSSH + "-" + sfx,
				Chain:   "input",
				Action:  "accept",
				Iif:     wan,
				Proto:   "tcp",
				DstPort: DefaultSSHPort,
				Comment: fmt.Sprintf("SSH TCP/%d %s（自动）", DefaultSSHPort, wan),
				Enabled: true,
				System:  true,
			})
		}
		if vpn.OCServEnabled && vpn.OCServTCP > 0 {
			out = append(out, FilterRule{
				ID: autoIDInputOcservTCP + "-" + sfx, Chain: "input", Action: "accept",
				Iif: wan, Proto: "tcp", DstPort: vpn.OCServTCP,
				Comment: fmt.Sprintf("OpenConnect TCP %s（自动）", wan), Enabled: true, System: true,
			})
		}
		if vpn.OCServEnabled && vpn.OCServUDP > 0 {
			out = append(out, FilterRule{
				ID: autoIDInputOcservUDP + "-" + sfx, Chain: "input", Action: "accept",
				Iif: wan, Proto: "udp", DstPort: vpn.OCServUDP,
				Comment: fmt.Sprintf("OpenConnect UDP %s（自动）", wan), Enabled: true, System: true,
			})
		}
		for _, p := range vpn.WGPorts {
			if p <= 0 {
				continue
			}
			out = append(out, FilterRule{
				ID:      fmt.Sprintf("auto-input-wg-%d-%s", p, sfx),
				Chain:   "input",
				Action:  "accept",
				Iif:     wan,
				Proto:   "udp",
				DstPort: p,
				Comment: fmt.Sprintf("WireGuard UDP/%d %s（自动）", p, wan),
				Enabled: true,
				System:  true,
			})
		}
		if vpn.SNMPEnabled {
			port := vpn.SNMPPort
			if port <= 0 {
				port = 161
			}
			nets := vpn.SNMPAllowedNetworks
			if len(nets) == 0 {
				nets = []string{"0.0.0.0/0"}
			}
			for i, cidr := range nets {
				cidr = strings.TrimSpace(cidr)
				if cidr == "" || cidr == "127.0.0.1/32" {
					continue
				}
				r := FilterRule{
					ID:      fmt.Sprintf("%s-%d-%s-%d", autoIDInputSNMPPrefix, port, sfx, i),
					Chain:   "input",
					Action:  "accept",
					Iif:     wan,
					Proto:   "udp",
					DstPort: port,
					Comment: fmt.Sprintf("SNMP UDP/%d %s（自动）", port, wan),
					Enabled: true,
					System:  true,
				}
				if !IsAnyCIDR(cidr) {
					r.SrcAddr = cidr
				}
				out = append(out, r)
			}
		}
		for _, ep := range vpn.LVSEndpoints {
			if ep.Port <= 0 || strings.TrimSpace(ep.VIP) == "" || strings.TrimSpace(ep.Proto) == "" {
				continue
			}
			id := strings.TrimSpace(ep.VSID)
			if id == "" {
				id = "vs"
			}
			out = append(out, FilterRule{
				ID:      fmt.Sprintf("%s-%s-%s-%s", autoIDInputLVSPrefix, id, ep.Proto, sfx),
				Chain:   "input",
				Action:  "accept",
				Iif:     wan,
				Proto:   ep.Proto,
				DstAddr: ep.VIP + "/32",
				DstPort: ep.Port,
				Comment: fmt.Sprintf("LVS VIP %s %s/%d %s（自动）", ep.VIP, strings.ToUpper(ep.Proto), ep.Port, wan),
				Enabled: true,
				System:  true,
			})
		}
		out = append(out, FilterRule{
			ID: autoIDInputWanDrop + "-" + sfx, Chain: "input", Action: "drop",
			Iif: wan, Comment: fmt.Sprintf("WAN 入站默认丢弃 %s（自动）", wan), Enabled: true, System: true,
		})
	}
	return out
}

func parsePortInt(s string) int {
	var n int
	_, _ = fmt.Sscanf(strings.TrimSpace(s), "%d", &n)
	if n <= 0 || n > 65535 {
		return 8080
	}
	return n
}

func filterRulesEqual(a, b []FilterRule) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !autoRuleEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func autoRuleEqual(a, b FilterRule) bool {
	return a.ID == b.ID && a.Chain == b.Chain && a.Action == b.Action &&
		a.Iif == b.Iif && a.Oif == b.Oif && a.Proto == b.Proto &&
		a.SrcAddr == b.SrcAddr && a.DstAddr == b.DstAddr &&
		a.SrcAlias == b.SrcAlias && a.DstAlias == b.DstAlias &&
		a.SrcPort == b.SrcPort && a.DstPort == b.DstPort &&
		a.IPVersion == b.IPVersion &&
		a.Comment == b.Comment && a.Enabled == b.Enabled && a.System == b.System
}

// splitAutoInputByDrop 将 input 自动规则拆为放行段与 WAN 默认丢弃段。
func splitAutoInputByDrop(rules []FilterRule) (accepts, drops []FilterRule) {
	for _, r := range rules {
		if strings.HasPrefix(strings.TrimSpace(r.ID), autoIDInputWanDrop) {
			drops = append(drops, r)
		} else {
			accepts = append(accepts, r)
		}
	}
	return accepts, drops
}

// BuildAutoHairpinInputRules 内网访问公网 IP 上本机服务时的 input 放行（NAT reflection / 环回）。
func BuildAutoHairpinInputRules(wanDevs []string, adminPort string, vpn AutoInputVPN, forwards []WanPortForward, devLAN string, resolver HairpinAddrResolver) []FilterRule {
	devLAN = strings.TrimSpace(devLAN)
	if devLAN == "" || resolver.PrimaryIPv4 == nil {
		return nil
	}
	port := parsePortInt(adminPort)
	var out []FilterRule
	wanIP := func(dev string) string {
		ip, err := resolver.PrimaryIPv4(dev)
		if err != nil {
			return ""
		}
		return ip
	}
	mirror := func(id, wan, addrFam, wanAddr, proto string, dstPort int, comment string) {
		if wanAddr == "" || dstPort <= 0 {
			return
		}
		r := FilterRule{
			ID:      id,
			Chain:   "input",
			Action:  "accept",
			Iif:     devLAN,
			Proto:   proto,
			DstAddr: wanAddr,
			DstPort: dstPort,
			Comment: comment,
			Enabled: true,
			System:  true,
		}
		if addrFam == "ipv6" {
			r.IPVersion = "ipv6"
		}
		out = append(out, r)
	}
	for _, wan := range wanDevs {
		wan = strings.TrimSpace(wan)
		if wan == "" {
			continue
		}
		sfx := wan
		mirror(
			autoIDInputHairpinAdmin+"-"+sfx, wan, "ipv4", wanIP(wan), "tcp", port,
			fmt.Sprintf("内网访问公网 IP 管理口 %s（自动）", wan),
		)
		if port != DefaultSSHPort {
			mirror(
				autoIDInputHairpinSSH+"-"+sfx, wan, "ipv4", wanIP(wan), "tcp", DefaultSSHPort,
				fmt.Sprintf("内网访问公网 IP SSH %s（自动）", wan),
			)
		}
		if vpn.OCServEnabled && vpn.OCServTCP > 0 {
			mirror(
				autoIDInputOcservTCP+"-hairpin-"+sfx, wan, "ipv4", wanIP(wan), "tcp", vpn.OCServTCP,
				fmt.Sprintf("内网访问公网 IP OpenConnect TCP %s（自动）", wan),
			)
		}
		if vpn.OCServEnabled && vpn.OCServUDP > 0 {
			mirror(
				autoIDInputOcservUDP+"-hairpin-"+sfx, wan, "ipv4", wanIP(wan), "udp", vpn.OCServUDP,
				fmt.Sprintf("内网访问公网 IP OpenConnect UDP %s（自动）", wan),
			)
		}
		for _, p := range vpn.WGPorts {
			if p <= 0 {
				continue
			}
			mirror(
				fmt.Sprintf("auto-input-wg-hairpin-%d-%s", p, sfx), wan, "ipv4", wanIP(wan), "udp", p,
				fmt.Sprintf("内网访问公网 IP WireGuard UDP/%d %s（自动）", p, wan),
			)
		}
	}
	primary4 := resolver.PrimaryIPv4
	primary6 := resolver.PrimaryIPv6
	for _, f := range forwards {
		iface := strings.TrimSpace(f.Interface)
		if iface == "" {
			continue
		}
		wanAddr := HairpinMatchAddr(f, iface, primary4, primary6)
		if wanAddr == "" {
			continue
		}
		comment := strings.TrimSpace(f.Comment)
		if comment == "" {
			comment = f.ID
		}
		src := strings.TrimSpace(f.SrcAddr)
		for _, proto := range ForwardProtos(f.Proto) {
			r := FilterRule{
				ID:        fmt.Sprintf("%s-%s-%s", autoIDInputHairpinFwd, f.ID, proto),
				Chain:     "input",
				Action:    "accept",
				Iif:       devLAN,
				Proto:     proto,
				DstAddr:   wanAddr,
				DstPort:   f.DstPort,
				Comment:   fmt.Sprintf("内网访问公网 IP 端口转发 %s（自动）", comment),
				Enabled:   true,
				System:    true,
				IPVersion: f.IPVersion,
			}
			if !IsAnyCIDR(src) {
				r.SrcAddr = src
			}
			out = append(out, r)
		}
	}
	return out
}

// SyncAutoFilterRules 合并用户规则与自动规则。
// forward：用户 → 端口转发 WAN→LAN → 端口转发回流 LAN→LAN。
// input：公网 IP 环回 → 自动放行(admin/SSH/VPN) → 用户 → 自动 WAN 丢弃。
func SyncAutoFilterRules(rules []FilterRule, wanDevs []string, adminPort string, vpn AutoInputVPN, forwards []WanPortForward, lvs LVSState, devLAN, defaultWAN string, resolver HairpinAddrResolver) ([]FilterRule, bool) {
	desiredFwd := BuildAutoForwardFilterRules(forwards, devLAN)
	desiredLVSFwd := BuildAutoLVSForwardFilterRules(lvs, devLAN, defaultWAN)
	desiredLVSRSInput := BuildAutoLVSRSInputRules(lvs, devLAN)
	desiredHairpinFwd := BuildAutoHairpinForwardFilterRules(forwards, devLAN, resolver.IsLocalIP)
	hairpinInput := BuildAutoHairpinInputRules(wanDevs, adminPort, vpn, forwards, devLAN, resolver)
	autoInputAccept, autoInputDrop := splitAutoInputByDrop(BuildAutoInputRules(wanDevs, adminPort, vpn))
	var userFwd, userInput []FilterRule
	for _, r := range rules {
		if IsAutoManagedRule(r) {
			continue
		}
		if strings.ToLower(strings.TrimSpace(r.Chain)) == "input" {
			userInput = append(userInput, r)
		} else {
			userFwd = append(userFwd, r)
		}
	}
	merged := append(append(append(append(append(append(append(append(
		append([]FilterRule{}, userFwd...),
		desiredFwd...),
		desiredLVSFwd...),
		desiredHairpinFwd...),
		hairpinInput...),
		autoInputAccept...),
		desiredLVSRSInput...),
		userInput...),
		autoInputDrop...)
	return merged, !filterRulesEqual(merged, rules)
}
