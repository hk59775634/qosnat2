package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

// AliasSet nft 对象组（ipv4 地址/网段集合；fqdn/geoip/asn 解析后仍以 ipv4 members 写入 nft；port 为端口集合）
type AliasSet struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // ipv4_addr | fqdn | asn | geoip | port
	ASN          int      `json:"asn,omitempty"`
	Members      []string `json:"members"`
	Domains      []string `json:"domains,omitempty"`           // type=fqdn：待解析域名列表
	Countries    []string `json:"countries,omitempty"`         // type=geoip：ISO 国家码
	URL          string   `json:"url,omitempty"`              // 可选：ipv4_addr 拉取 CIDR；fqdn 拉取域名列表
	URLFetchedAt string   `json:"url_fetched_at,omitempty"`   // 上次 URL/GeoIP 拉取时间（RFC3339）
	ResolvedAt   string   `json:"resolved_at,omitempty"`      // 上次 FQDN 解析时间（RFC3339）
	Comment      string   `json:"comment,omitempty"`
}

// NormalizeAlias 校验别名
func NormalizeAlias(a *AliasSet) error {
	if a == nil {
		return fmt.Errorf("alias nil")
	}
	name := strings.TrimSpace(a.Name)
	if name == "" {
		b := make([]byte, 4)
		_, _ = rand.Read(b)
		name = "alias_" + hex.EncodeToString(b)
	}
	if !isValidAliasName(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	a.Name = name
	typ := strings.ToLower(strings.TrimSpace(a.Type))
	if typ == "" {
		typ = "ipv4_addr"
	}
	switch typ {
	case "ipv4_addr", "fqdn", "asn", "geoip", "port":
	default:
		return fmt.Errorf("type must be ipv4_addr, fqdn, asn, geoip, or port")
	}
	a.Type = typ
	a.Comment = strings.TrimSpace(a.Comment)
	a.URL = strings.TrimSpace(a.URL)
	a.ResolvedAt = strings.TrimSpace(a.ResolvedAt)
	a.URLFetchedAt = strings.TrimSpace(a.URLFetchedAt)

	if typ == "port" {
		a.Domains = nil
		a.Countries = nil
		a.ASN = 0
		a.ResolvedAt = ""
		a.URL = ""
		a.URLFetchedAt = ""
		var members []string
		seen := map[string]struct{}{}
		for _, m := range a.Members {
			parts, err := ParsePortSpec(m)
			if err != nil {
				return fmt.Errorf("member %q: %w", strings.TrimSpace(m), err)
			}
			for _, p := range parts {
				if _, ok := seen[p]; ok {
					continue
				}
				seen[p] = struct{}{}
				members = append(members, p)
			}
		}
		if len(members) == 0 {
			return fmt.Errorf("members required for port alias")
		}
		a.Members = members
		return nil
	}

	if typ == "geoip" {
		a.Domains = nil
		a.ASN = 0
		a.ResolvedAt = ""
		a.URL = ""
		var ccs []string
		seen := map[string]struct{}{}
		for _, c := range a.Countries {
			cc, err := NormalizeGeoCountry(c)
			if err != nil {
				return fmt.Errorf("country %q: %w", strings.TrimSpace(c), err)
			}
			if _, ok := seen[cc]; ok {
				continue
			}
			seen[cc] = struct{}{}
			ccs = append(ccs, cc)
		}
		if len(ccs) == 0 {
			return fmt.Errorf("countries required for geoip alias")
		}
		if len(ccs) > geoipMaxCountries {
			return fmt.Errorf("too many countries (max %d)", geoipMaxCountries)
		}
		a.Countries = ccs
		var members []string
		for _, m := range a.Members {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if err := ValidateIPv4OrCIDR(m); err != nil {
				return fmt.Errorf("member %q: %w", m, err)
			}
			members = append(members, m)
		}
		a.Members = members
		return nil
	}

	if typ == "asn" {
		a.Domains = nil
		a.Countries = nil
		a.ResolvedAt = ""
		if a.ASN < 0 {
			return fmt.Errorf("asn must be >= 0")
		}
		var members []string
		for _, m := range a.Members {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if err := ValidateIPv4OrCIDR(m); err != nil {
				return fmt.Errorf("member %q: %w", m, err)
			}
			members = append(members, m)
		}
		a.Members = members
		if a.URL != "" {
			if _, err := url.Parse(a.URL); err != nil {
				return fmt.Errorf("url: %w", err)
			}
			if len(members) == 0 {
				a.Members = nil
				return nil
			}
		}
		if len(members) == 0 && a.URL == "" {
			return fmt.Errorf("members required for asn alias (or set url to fetch)")
		}
		return nil
	}

	if typ == "fqdn" {
		a.Countries = nil
		a.ASN = 0
		if a.URL != "" {
			if _, err := url.Parse(a.URL); err != nil {
				return fmt.Errorf("url: %w", err)
			}
		}
		var domains []string
		seen := map[string]struct{}{}
		for _, d := range a.Domains {
			nd, err := NormalizeFQDN(d)
			if err != nil {
				return fmt.Errorf("domain %q: %w", strings.TrimSpace(d), err)
			}
			if _, ok := seen[nd]; ok {
				continue
			}
			seen[nd] = struct{}{}
			domains = append(domains, nd)
		}
		if len(domains) == 0 && a.URL == "" {
			return fmt.Errorf("domains required for fqdn alias (or set url to fetch)")
		}
		if len(domains) > fqdnMaxDomains {
			return fmt.Errorf("too many domains (max %d)", fqdnMaxDomains)
		}
		a.Domains = domains
		var members []string
		for _, m := range a.Members {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if err := ValidateIPv4OrCIDR(m); err != nil {
				return fmt.Errorf("member %q: %w", m, err)
			}
			members = append(members, m)
		}
		a.Members = members
		return nil
	}

	a.Domains = nil
	a.Countries = nil
	a.ASN = 0
	a.ResolvedAt = ""
	var members []string
	for _, m := range a.Members {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if err := ValidateIPv4OrCIDR(m); err != nil {
			return fmt.Errorf("member %q: %w", m, err)
		}
		members = append(members, m)
	}
	a.Members = members
	if a.URL != "" {
		if _, err := url.Parse(a.URL); err != nil {
			return fmt.Errorf("url: %w", err)
		}
		if len(members) == 0 {
			a.Members = nil
			return nil
		}
	}
	if len(members) == 0 {
		return fmt.Errorf("members required (or set url to fetch)")
	}
	return nil
}

// ValidateAliasName 校验防火墙别名引用名（字母数字下划线，最长 32）。
func ValidateAliasName(s string) error {
	if !isValidAliasName(strings.TrimSpace(s)) {
		return fmt.Errorf("invalid alias name %q", s)
	}
	return nil
}

func isValidAliasName(s string) bool {
	if len(s) == 0 || len(s) > 32 {
		return false
	}
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '_' {
			continue
		}
		return false
	}
	return true
}

// NftSetName 生成 nft set 标识符
func (a AliasSet) NftSetName() string {
	return "alias_" + a.Name
}

// AliasReferencedByRules 是否有防火墙规则引用该别名。
func AliasReferencedByRules(rules []FilterRule, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, r := range rules {
		if strings.TrimSpace(r.SrcAlias) == name || strings.TrimSpace(r.DstAlias) == name {
			return true
		}
		if strings.TrimSpace(r.SrcPortAlias) == name || strings.TrimSpace(r.DstPortAlias) == name {
			return true
		}
	}
	return false
}

// AliasReferencedByEgress 是否有出站策略引用该别名。
func AliasReferencedByEgress(policies []EgressPolicy, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	for _, p := range policies {
		if strings.TrimSpace(p.SrcAlias) == name || strings.TrimSpace(p.DstAlias) == name {
			return true
		}
	}
	return false
}

// ValidateFilterRuleAliases 校验规则引用的地址别名存在且可渲染为 nft set。
func ValidateFilterRuleAliases(r FilterRule, aliases []AliasSet) error {
	byName := make(map[string]AliasSet, len(aliases))
	for _, a := range aliases {
		byName[a.Name] = a
	}
	check := func(field, name string) error {
		name = strings.TrimSpace(name)
		if name == "" {
			return nil
		}
		a, ok := byName[name]
		if !ok {
			return fmt.Errorf("%s: alias %q not found", field, name)
		}
		typ := strings.ToLower(strings.TrimSpace(a.Type))
		if typ == "port" {
			return fmt.Errorf("%s: alias %q is a port alias; use src_port_alias/dst_port_alias", field, name)
		}
		return nil
	}
	if err := check("src_alias", r.SrcAlias); err != nil {
		return err
	}
	return check("dst_alias", r.DstAlias)
}
