package store

import "net"

// IPInCIDR 判断 IPv4 是否落在 cidr 内
func IPInCIDR(ip, cidr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	return n.Contains(p)
}

// ProfileByCIDR 在列表中查找 profile
func ProfileByCIDR(profiles []ProfileEntry, cidr string) (ProfileEntry, bool) {
	for _, p := range profiles {
		if p.CIDR == cidr {
			return p, true
		}
	}
	return ProfileEntry{}, false
}
