package store

// CloneDHCP deep-copies DHCP / dnsmasq LAN service state.
func CloneDHCP(d DHCPState) DHCPState {
	out := d
	out.DNSServers = append([]string(nil), d.DNSServers...)
	out.UpstreamDNS = append([]string(nil), d.UpstreamDNS...)
	out.TrustedDNS = append([]string(nil), d.TrustedDNS...)
	out.UntrustedDNS = append([]string(nil), d.UntrustedDNS...)
	if len(d.StaticLeases) > 0 {
		out.StaticLeases = make([]DHCPStaticLease, len(d.StaticLeases))
		for i, sl := range d.StaticLeases {
			out.StaticLeases[i] = sl
			if len(sl.DNSServers) > 0 {
				out.StaticLeases[i].DNSServers = append([]string(nil), sl.DNSServers...)
			}
		}
	}
	return out
}
