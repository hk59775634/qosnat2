package store

import (
	"fmt"
	"net"
	"strings"
	"unicode"
)

// 接口页派生的策略路由 WanLink / EgressPolicy ID 前缀。
const (
	IfacePolicyWanLinkPrefix = "iface-pr-"
	IfacePolicyEgressPrefix  = "auto-iface-pr-"
	IfacePolicyEgressPrio    = 90
	IfacePolicyWanMetric     = 500
	IfacePolicyWanTier       = 99
)

// IsIfacePolicyWanLink 是否为接口页策略路由自动托管的 WanLink。
func IsIfacePolicyWanLink(w WanLink) bool {
	return w.IfaceManaged || strings.HasPrefix(w.ID, IfacePolicyWanLinkPrefix)
}

// IsIfacePolicyEgress 是否为接口页策略路由自动托管的出站策略。
func IsIfacePolicyEgress(p EgressPolicy) bool {
	return strings.HasPrefix(p.ID, IfacePolicyEgressPrefix)
}

func sanitizeIfacePolicyIDPart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "x"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "x"
	}
	return out
}

// IfacePolicyWanLinkID 派生稳定 WanLink ID。
func IfacePolicyWanLinkID(device string) string {
	return IfacePolicyWanLinkPrefix + sanitizeIfacePolicyIDPart(device)
}

// IfacePolicyEgressID 按设备 + 主机 IP 派生稳定 EgressPolicy ID。
func IfacePolicyEgressID(device, hostIP string) string {
	return IfacePolicyEgressPrefix + sanitizeIfacePolicyIDPart(device) + "-" + strings.ReplaceAll(strings.TrimSpace(hostIP), ".", "-")
}

// IfaceHostIPv4s 从托管地址列表提取主机 IPv4（CIDR 取前缀地址）。
func IfaceHostIPv4s(ipv4 []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, raw := range ipv4 {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		var host string
		if strings.Contains(raw, "/") {
			ip, _, err := net.ParseCIDR(raw)
			if err != nil {
				continue
			}
			v4 := ip.To4()
			if v4 == nil {
				continue
			}
			host = v4.String()
		} else {
			ip := net.ParseIP(raw)
			if ip == nil || ip.To4() == nil {
				continue
			}
			host = ip.To4().String()
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		out = append(out, host)
	}
	return out
}

// ValidateIfacePolicyRouting 校验启用策略路由时的网关与静态地址。
func ValidateIfacePolicyRouting(ic IfaceConfig) error {
	if !ic.PolicyRouting {
		gw := strings.TrimSpace(ic.Gateway)
		if gw == "" {
			return nil
		}
		if err := ValidateIPv4OrCIDR(gw); err != nil {
			return fmt.Errorf("gateway: %w", err)
		}
		if strings.Contains(gw, "/") {
			return fmt.Errorf("gateway must be host address, not cidr")
		}
		return nil
	}
	if ic.DHCP4 {
		return fmt.Errorf("policy_routing requires static IPv4 (disable dhcp4)")
	}
	gw := strings.TrimSpace(ic.Gateway)
	if gw == "" {
		return fmt.Errorf("gateway required when policy_routing is enabled")
	}
	if err := ValidateIPv4OrCIDR(gw); err != nil {
		return fmt.Errorf("gateway: %w", err)
	}
	if strings.Contains(gw, "/") {
		return fmt.Errorf("gateway must be host address, not cidr")
	}
	if len(IfaceHostIPv4s(ic.IPv4)) == 0 {
		return fmt.Errorf("policy_routing requires at least one static IPv4 address")
	}
	return nil
}

// SyncIfacePolicyRouting 根据 IfaceConfig 重建接口派生的 WanLink + 源地址 EgressPolicy。
// 不写入 main default，仅 PolicyOnly 表 + from <IP/32> lookup（由 SyncEgressRoutes / policyroute 落地）。
func SyncIfacePolicyRouting(st *State) {
	if st == nil {
		return
	}
	keepLinks := make([]WanLink, 0, len(st.Network.WanLinks))
	for _, w := range st.Network.WanLinks {
		if IsIfacePolicyWanLink(w) {
			continue
		}
		keepLinks = append(keepLinks, w)
	}
	keepPolicies := make([]EgressPolicy, 0, len(st.Network.EgressPolicies))
	for _, p := range st.Network.EgressPolicies {
		if IsIfacePolicyEgress(p) {
			continue
		}
		keepPolicies = append(keepPolicies, p)
	}

	for _, ic := range st.Network.Ifaces {
		if !ic.PolicyRouting {
			continue
		}
		gw := strings.TrimSpace(ic.Gateway)
		dev := strings.TrimSpace(ic.Device)
		if gw == "" || dev == "" || ic.DHCP4 {
			continue
		}
		hosts := IfaceHostIPv4s(ic.IPv4)
		if len(hosts) == 0 {
			continue
		}
		wanID := IfacePolicyWanLinkID(dev)
		keepLinks = append(keepLinks, WanLink{
			ID:           wanID,
			Name:         "Iface PR · " + dev,
			Device:       dev,
			Gateway:      gw,
			Metric:       IfacePolicyWanMetric,
			Tier:         IfacePolicyWanTier,
			PolicyOnly:   true,
			Enabled:      true,
			IfaceManaged: true,
		})
		for _, host := range hosts {
			keepPolicies = append(keepPolicies, EgressPolicy{
				ID:        IfacePolicyEgressID(dev, host),
				Name:      "Iface PR · " + dev + " · " + host,
				SrcCIDR:   host + "/32",
				WanLinkID: wanID,
				NoSNAT:    true,
				Priority:  IfacePolicyEgressPrio,
				Enabled:   true,
			})
		}
	}
	st.Network.WanLinks = keepLinks
	st.Network.EgressPolicies = keepPolicies
}
