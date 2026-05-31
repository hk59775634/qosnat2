package store

// CloneDHCP deep-copies DHCP / dnsmasq LAN service state.
func CloneDHCP(d DHCPState) DHCPState {
	out := d
	out.DNSServers = append([]string(nil), d.DNSServers...)
	out.UpstreamDNS = append([]string(nil), d.UpstreamDNS...)
	if len(d.StaticLeases) > 0 {
		out.StaticLeases = append([]DHCPStaticLease(nil), d.StaticLeases...)
	}
	return out
}
