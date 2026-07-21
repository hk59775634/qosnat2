package store

import (
	"fmt"
	"net"
	"strings"
	"time"
)

const (
	geoipMaxCountries = 32
	geoipMaxMembers   = 200000
	// GeoIPDenyBaseURL ipdeny.com 国家 CIDR 列表（每国一文件）。
	GeoIPDenyBaseURL = "https://www.ipdeny.com/ipblocks/data/countries"
)

// NormalizeGeoCountry 规范化 ISO 3166-1 alpha-2 国家码。
func NormalizeGeoCountry(cc string) (string, error) {
	cc = strings.ToLower(strings.TrimSpace(cc))
	if len(cc) != 2 {
		return "", fmt.Errorf("country code must be 2 letters")
	}
	for _, c := range cc {
		if c < 'a' || c > 'z' {
			return "", fmt.Errorf("invalid country code %q", cc)
		}
	}
	return cc, nil
}

// FetchGeoIPCountryCIDRs 拉取单国 IPv4 CIDR 列表。
func FetchGeoIPCountryCIDRs(cc string) ([]string, error) {
	cc, err := NormalizeGeoCountry(cc)
	if err != nil {
		return nil, err
	}
	return FetchCIDRListFromURL(GeoIPDenyBaseURL + "/" + cc + ".zone")
}

// RefreshGeoIPAlias 按 Countries 拉取并合并 members。
func RefreshGeoIPAlias(a *AliasSet) error {
	if a == nil {
		return fmt.Errorf("alias nil")
	}
	if strings.ToLower(strings.TrimSpace(a.Type)) != "geoip" {
		return fmt.Errorf("not a geoip alias")
	}
	if len(a.Countries) == 0 {
		return fmt.Errorf("countries required")
	}
	seen := map[string]struct{}{}
	var members []string
	for _, cc := range a.Countries {
		list, err := FetchGeoIPCountryCIDRs(cc)
		if err != nil {
			return fmt.Errorf("country %s: %w", cc, err)
		}
		for _, m := range list {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if _, ok := seen[m]; ok {
				continue
			}
			// 宽松校验：跳过非法行
			if ip := net.ParseIP(m); ip != nil && ip.To4() != nil {
				m = m + "/32"
			} else if _, _, err := net.ParseCIDR(m); err != nil {
				continue
			}
			seen[m] = struct{}{}
			members = append(members, m)
			if len(members) > geoipMaxMembers {
				return fmt.Errorf("too many prefixes (max %d)", geoipMaxMembers)
			}
		}
	}
	a.Members = members
	a.URLFetchedAt = time.Now().UTC().Format(time.RFC3339)
	return nil
}
