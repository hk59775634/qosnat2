package store

// CloneDHCP deep-copies DHCP / dnsmasq LAN service state.
func CloneDHCP(d DHCPState) DHCPState {
	out := d
	out.DNSServers = append([]string(nil), d.DNSServers...)
	out.UpstreamDNS = append([]string(nil), d.UpstreamDNS...)
	out.TrustedDNS = append([]string(nil), d.TrustedDNS...)
	out.UntrustedDNS = append([]string(nil), d.UntrustedDNS...)
	if len(d.StaticLeases) > 0 {
		out.StaticLeases = append([]DHCPStaticLease(nil), d.StaticLeases...)
	}
	return out
}
