package store

import (
	"context"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	fqdnResolveTimeout   = 5 * time.Second
	fqdnResolveWorkers   = 16
	fqdnMaxDomains       = 500
	fqdnMaxResolvedAddrs = 10000
)

// NormalizeFQDN 规范化并校验主机名（不支持通配符）。
func NormalizeFQDN(s string) (string, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.TrimSuffix(s, ".")
	if s == "" {
		return "", fmt.Errorf("empty domain")
	}
	if len(s) > 253 {
		return "", fmt.Errorf("domain too long")
	}
	if strings.Contains(s, "*") {
		return "", fmt.Errorf("wildcard domains are not supported")
	}
	if strings.ContainsAny(s, " \t\n\r/\\:@?#[]") {
		return "", fmt.Errorf("invalid characters in domain")
	}
	labels := strings.Split(s, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("domain must have at least two labels")
	}
	for _, label := range labels {
		if len(label) == 0 || len(label) > 63 {
			return "", fmt.Errorf("invalid domain label %q", label)
		}
		if label[0] == '-' || label[len(label)-1] == '-' {
			return "", fmt.Errorf("invalid domain label %q", label)
		}
		for i := 0; i < len(label); i++ {
			c := label[i]
			ok := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
			if !ok {
				return "", fmt.Errorf("invalid domain label %q", label)
			}
		}
	}
	return s, nil
}

// ResolveDomainsToIPv4CIDRs 将域名解析为 IPv4 /32 列表（系统 DNS）。
func ResolveDomainsToIPv4CIDRs(domains []string) (members []string, unresolved []string, err error) {
	if len(domains) == 0 {
		return nil, nil, fmt.Errorf("no domains")
	}
	type result struct {
		domain string
		ips    []string
	}
	jobs := make(chan string)
	out := make(chan result, len(domains))
	var wg sync.WaitGroup
	workers := fqdnResolveWorkers
	if workers > len(domains) {
		workers = len(domains)
	}
	resolver := &net.Resolver{PreferGo: true}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for domain := range jobs {
				ctx, cancel := context.WithTimeout(context.Background(), fqdnResolveTimeout)
				ips, lookupErr := resolver.LookupIP(ctx, "ip4", domain)
				cancel()
				var cidrs []string
				if lookupErr == nil {
					seen := map[string]struct{}{}
					for _, ip := range ips {
						v4 := ip.To4()
						if v4 == nil {
							continue
						}
						c := v4.String() + "/32"
						if _, ok := seen[c]; ok {
							continue
						}
						seen[c] = struct{}{}
						cidrs = append(cidrs, c)
					}
				}
				out <- result{domain: domain, ips: cidrs}
			}
		}()
	}
	go func() {
		for _, d := range domains {
			jobs <- d
		}
		close(jobs)
		wg.Wait()
		close(out)
	}()

	seenMember := map[string]struct{}{}
	for r := range out {
		if len(r.ips) == 0 {
			unresolved = append(unresolved, r.domain)
			continue
		}
		for _, c := range r.ips {
			if _, ok := seenMember[c]; ok {
				continue
			}
			seenMember[c] = struct{}{}
			members = append(members, c)
			if len(members) >= fqdnMaxResolvedAddrs {
				sort.Strings(members)
				sort.Strings(unresolved)
				return members, unresolved, nil
			}
		}
	}
	sort.Strings(members)
	sort.Strings(unresolved)
	if len(members) == 0 {
		return nil, unresolved, fmt.Errorf("no IPv4 addresses resolved")
	}
	return members, unresolved, nil
}

// RefreshAliasFromFQDN 解析 Domains 并更新 Members / ResolvedAt。
func RefreshAliasFromFQDN(a *AliasSet) error {
	if a == nil {
		return fmt.Errorf("alias nil")
	}
	if strings.ToLower(strings.TrimSpace(a.Type)) != "fqdn" {
		return fmt.Errorf("alias %q is not type fqdn", a.Name)
	}
	if len(a.Domains) == 0 {
		return fmt.Errorf("alias %q has no domains", a.Name)
	}
	members, unresolved, err := ResolveDomainsToIPv4CIDRs(a.Domains)
	if err != nil {
		if len(unresolved) > 0 {
			return fmt.Errorf("%w (unresolved: %s)", err, strings.Join(unresolved, ", "))
		}
		return err
	}
	a.Members = members
	now := time.Now().UTC().Format(time.RFC3339)
	a.ResolvedAt = now
	if len(unresolved) > 0 {
		return &FQDNPartialResolveError{Unresolved: unresolved, Members: members}
	}
	return nil
}

// FQDNPartialResolveError 部分域名解析失败，但已有可用 members（调用方可选择接受）。
type FQDNPartialResolveError struct {
	Unresolved []string
	Members    []string
}

func (e *FQDNPartialResolveError) Error() string {
	return fmt.Sprintf("partial resolve; unresolved: %s", strings.Join(e.Unresolved, ", "))
}

// AcceptFQDNPartial 若 err 为部分成功，写入 members 并返回警告文案；否则原样返回 err。
func AcceptFQDNPartial(a *AliasSet, err error) (warn string, fatal error) {
	if err == nil {
		return "", nil
	}
	if pe, ok := err.(*FQDNPartialResolveError); ok {
		if a != nil {
			a.Members = pe.Members
			a.ResolvedAt = time.Now().UTC().Format(time.RFC3339)
		}
		return pe.Error(), nil
	}
	return "", err
}

// AliasNeedsDynamicRefresh 是否可通过 URL、FQDN 或 GeoIP 自动刷新。
func AliasNeedsDynamicRefresh(a AliasSet) bool {
	typ := strings.ToLower(strings.TrimSpace(a.Type))
	if typ == "geoip" && len(a.Countries) > 0 {
		return true
	}
	if typ == "fqdn" && (len(a.Domains) > 0 || strings.TrimSpace(a.URL) != "") {
		return true
	}
	if strings.TrimSpace(a.URL) != "" {
		return true
	}
	return false
}

// RefreshAliasDynamic 按类型刷新 URL、FQDN 或 GeoIP 别名。
func RefreshAliasDynamic(a *AliasSet) (warn string, err error) {
	if a == nil {
		return "", fmt.Errorf("alias nil")
	}
	typ := strings.ToLower(strings.TrimSpace(a.Type))
	if typ == "geoip" {
		if err := RefreshGeoIPAlias(a); err != nil {
			return "", err
		}
		return "", nil
	}
	if typ == "fqdn" {
		if strings.TrimSpace(a.URL) != "" {
			if err := RefreshAliasDomainsFromURL(a); err != nil {
				return "", err
			}
		}
		if len(a.Domains) == 0 {
			return "", fmt.Errorf("alias %q has no domains after url fetch", a.Name)
		}
		err := RefreshAliasFromFQDN(a)
		return AcceptFQDNPartial(a, err)
	}
	if strings.TrimSpace(a.URL) != "" {
		if err := RefreshAliasFromURL(a); err != nil {
			return "", err
		}
		return "", nil
	}
	return "", fmt.Errorf("alias %q has no url or fqdn domains", a.Name)
}

// RefreshDynamicAliases 刷新所有 URL / FQDN 别名。
func RefreshDynamicAliases(aliases []AliasSet) (updated []AliasSet, warns []string) {
	updated = make([]AliasSet, len(aliases))
	copy(updated, aliases)
	for i := range updated {
		if !AliasNeedsDynamicRefresh(updated[i]) {
			continue
		}
		warn, err := RefreshAliasDynamic(&updated[i])
		if err != nil {
			warns = append(warns, fmt.Sprintf("%s: %v", updated[i].Name, err))
			continue
		}
		if warn != "" {
			warns = append(warns, fmt.Sprintf("%s: %s", updated[i].Name, warn))
		}
	}
	return updated, warns
}
