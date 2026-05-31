package store

// CloneNatIPv4 deep-copies IPv4 NAT state maps and slices.
func CloneNatIPv4(n NatIPv4State) NatIPv4State {
	out := NatIPv4State{
		PolicyRoutes:   append([]string(nil), n.PolicyRoutes...),
		SharedIPs:      append([]string(nil), n.SharedIPs...),
		StaticMappings: make(map[string]string, len(n.StaticMappings)),
		PrefixMappings: make(map[string]string, len(n.PrefixMappings)),
	}
	for k, v := range n.StaticMappings {
		out.StaticMappings[k] = v
	}
	for k, v := range n.PrefixMappings {
		out.PrefixMappings[k] = v
	}
	return out
}

// CloneNatState deep-copies NAT / NPTv6 / NAT64 configuration.
func CloneNatState(n NatState) NatState {
	out := n
	out.IPv4 = CloneNatIPv4(n.IPv4)
	if len(n.Nptv6Rules) > 0 {
		out.Nptv6Rules = append([]Nptv6Rule(nil), n.Nptv6Rules...)
	}
	return out
}

// CloneEgressPolicies copies egress policy slice.
func CloneEgressPolicies(p []EgressPolicy) []EgressPolicy {
	return append([]EgressPolicy(nil), p...)
}
