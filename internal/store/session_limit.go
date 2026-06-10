package store

import (
	"errors"
	"net"
	"strings"
)

var ErrMaxSessionsPerIPRange = errors.New("max_sessions_per_ip must be 0 (off) or 1-100000")

const (
	// SessionLimitSetSize nft dynamic set 容量（每源 IP 一条 ct count 元数据）。
	SessionLimitSetSize = 65535
)

// CollectSessionLimitCIDRs 聚合需做每 IP 出站 conntrack 上限的内网源网段：
// LAN DHCP、QoS 策略网段、NAT 策略路由、出站策略源网段、ocserv/WG 隧道网段等。
func CollectSessionLimitCIDRs(st State) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(c string) {
		c = normalizeSessionLimitCIDR(c)
		if c == "" || sessionLimitCIDRIgnored(c) {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}

	for _, c := range CollectMirredCIDRs(st.Shaper) {
		add(c)
	}
	for _, t := range st.Shaper.Tenants {
		for _, c := range t.CIDRs {
			add(c)
		}
	}
	for _, c := range st.Nat.IPv4.PolicyRoutes {
		add(c)
	}
	for _, c := range EgressPolicySourceMatchCIDRs(st.Network.EgressPolicies) {
		add(c)
	}
	if st.Nat.Nat64Enabled {
		add(st.Nat.Nat64Pool4)
	}
	for inner := range st.Nat.IPv4.StaticMappings {
		add(inner)
	}
	for inner := range st.Nat.IPv4.PrefixMappings {
		add(inner)
	}
	add(dhcpLANCIDR(st.DHCP))
	if st.VPN.OCServ.Enabled {
		add(ocservPoolCIDR(st.VPN.OCServ))
		for _, g := range st.VPN.OCServ.Groups {
			add(ipv4NetworkMaskCIDR(g.IPv4Network, g.IPv4Netmask))
		}
		for _, v := range st.VPN.OCServ.Vhosts {
			add(ipv4NetworkMaskCIDR(v.IPv4Network, v.IPv4Netmask))
		}
	}
	for _, wg := range st.VPN.WireGuards {
		if !wg.Enabled {
			continue
		}
		for _, c := range WireGuardMirredSrcCIDRs(wg.WireGuardState) {
			add(c)
		}
		for _, p := range wg.Peers {
			for _, c := range p.AllowedIPs {
				add(c)
			}
		}
	}
	return out
}

func normalizeSessionLimitCIDR(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return ""
	}
	if _, _, err := net.ParseCIDR(c); err == nil {
		return c
	}
	if ip := net.ParseIP(c); ip != nil && ip.To4() != nil {
		return ip.String() + "/32"
	}
	return ""
}

func sessionLimitCIDRIgnored(cidr string) bool {
	switch cidr {
	case "0.0.0.0/0", "::/0":
		return true
	}
	if ip, n, err := net.ParseCIDR(cidr); err == nil && ip != nil {
		if ones, _ := n.Mask.Size(); ip.Equal(net.IPv4zero) && ones == 0 {
			return true
		}
	}
	return false
}

func ipv4NetworkMaskCIDR(network, netmask string) string {
	network = strings.TrimSpace(network)
	if network == "" {
		return ""
	}
	if strings.Contains(network, "/") {
		return network
	}
	mask := strings.TrimSpace(netmask)
	if mask == "" {
		mask = "255.255.255.0"
	}
	ip := net.ParseIP(network)
	maskIP := net.ParseIP(mask)
	if ip == nil || maskIP == nil || ip.To4() == nil {
		return ""
	}
	m := net.IPMask(maskIP.To4())
	return (&net.IPNet{IP: ip.Mask(m), Mask: m}).String()
}

func dhcpLANCIDR(d DHCPState) string {
	if !d.Enabled {
		return ""
	}
	router := strings.TrimSpace(d.Router)
	if router == "" {
		router = strings.TrimSpace(d.RangeStart)
	}
	if router == "" {
		return ""
	}
	mask := strings.TrimSpace(d.Netmask)
	if mask == "" {
		mask = "255.255.255.0"
	}
	return ipv4NetworkMaskCIDR(router, mask)
}

// NormalizeMaxSessionsPerIP 0=关闭；否则 1–100000。
func NormalizeMaxSessionsPerIP(n int) (int, error) {
	if n == 0 {
		return 0, nil
	}
	if n < 1 || n > 100000 {
		return 0, ErrMaxSessionsPerIPRange
	}
	return n, nil
}
