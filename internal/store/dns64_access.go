package store

import (
	"net"
	"strings"
)

// CollectDNS64AccessAllow 合并用户配置与 VPN 隧道网段（供 Unbound access-control）
func CollectDNS64AccessAllow(st State) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(cidr string) {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			return
		}
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			if ip := net.ParseIP(cidr); ip != nil {
				if ip.To4() != nil {
					cidr = ip.String() + "/32"
				} else {
					cidr = ip.String() + "/128"
				}
			} else {
				return
			}
		}
		if _, ok := seen[cidr]; ok {
			return
		}
		seen[cidr] = struct{}{}
		out = append(out, cidr)
	}
	for _, c := range st.Nat.DNS64.AccessAllow {
		add(c)
	}
	for _, c := range wireGuardPoolCIDRs(st.VPN.WireGuards) {
		add(c)
	}
	if c := ocservPoolCIDR(st.VPN.OCServ); c != "" {
		add(c)
	}
	if len(out) == 0 {
		add("10.0.0.0/8")
		add("100.64.0.0/10")
		add("::1/128")
	}
	return out
}

func wireGuardPoolCIDRs(insts []WireGuardInstance) []string {
	var out []string
	for _, w := range insts {
		if !w.Enabled {
			continue
		}
		a := strings.TrimSpace(w.Address)
		if a == "" {
			continue
		}
		if _, n, err := net.ParseCIDR(a); err == nil {
			out = append(out, n.String())
			continue
		}
		if ip := net.ParseIP(a); ip != nil && ip.To4() != nil {
			out = append(out, ip.String()+"/32")
		}
	}
	return out
}

func ocservPoolCIDR(o OCServState) string {
	net4 := strings.TrimSpace(o.IPv4Network)
	mask := strings.TrimSpace(o.IPv4Netmask)
	if net4 == "" {
		return ""
	}
	if strings.Contains(net4, "/") {
		return net4
	}
	if mask == "" {
		mask = "255.255.255.0"
	}
	ip := net.ParseIP(net4)
	maskIP := net.ParseIP(mask)
	if ip == nil || maskIP == nil {
		return ""
	}
	m := net.IPMask(maskIP.To4())
	return (&net.IPNet{IP: ip.Mask(m), Mask: m}).String()
}
