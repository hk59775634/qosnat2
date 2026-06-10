package acme

import (
	"fmt"
	"net"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
)

// ResolveHTTP01LocalIPs 解析 HTTP-01 挑战应放行的本机 IPv4。
// 域名：取 DNS A 记录与本机 global IPv4 的交集；IP：校验该地址在本机。
func ResolveHTTP01LocalIPs(target string) ([]string, error) {
	target = strings.TrimSpace(target)
	if target == "" {
		return nil, fmt.Errorf("acme http-01 target required")
	}
	local, err := netif.CollectLocalGlobalIPv4()
	if err != nil {
		return nil, err
	}
	localSet := make(map[string]struct{}, len(local))
	for _, ip := range local {
		localSet[ip] = struct{}{}
	}
	if ip, err := NormalizeIP(target); err == nil {
		if _, ok := localSet[ip]; !ok {
			return nil, fmt.Errorf("IP %s is not configured on this host", ip)
		}
		return []string{ip}, nil
	}
	domain, err := NormalizeDomain(target)
	if err != nil {
		return nil, err
	}
	records, err := net.LookupIP(domain)
	if err != nil {
		return nil, fmt.Errorf("dns lookup %s: %w", domain, err)
	}
	seen := map[string]struct{}{}
	var out []string
	for _, rec := range records {
		ip4 := rec.To4()
		if ip4 == nil {
			continue
		}
		ip := ip4.String()
		if _, ok := localSet[ip]; !ok {
			continue
		}
		if _, dup := seen[ip]; dup {
			continue
		}
		seen[ip] = struct{}{}
		out = append(out, ip)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("domain %s does not resolve to any local IPv4 address", domain)
	}
	return out, nil
}
