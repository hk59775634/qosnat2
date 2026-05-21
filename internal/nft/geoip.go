package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/geoip"
	"github.com/hk59775634/qosnat2/internal/store"
)

func geoSetName(country string) string {
	return "geo_" + strings.ToLower(strings.TrimSpace(country))
}

func writeGeoIPSets(b *strings.Builder, rules []store.GeoIPRule) {
	for _, r := range rules {
		if !r.Enabled || r.Action != "drop" {
			continue
		}
		cidrs, err := geoip.LoadCIDRs(r.Country, r.CustomCIDRs)
		if err != nil || len(cidrs) == 0 {
			continue
		}
		name := geoSetName(r.Country)
		b.WriteString(fmt.Sprintf("    set %s {\n", name))
		b.WriteString("        type ipv4_addr\n")
		b.WriteString("        flags interval\n")
		b.WriteString("        elements = { ")
		for i, c := range cidrs {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(c)
		}
		b.WriteString(" }\n")
		b.WriteString("    }\n\n")
	}
}

func writeGeoIPRules(b *strings.Builder, chain, wan string, rules []store.GeoIPRule) {
	if chain != "forward" {
		return
	}
	for _, r := range rules {
		if !r.Enabled || r.Action != "drop" {
			continue
		}
		cidrs, err := geoip.LoadCIDRs(r.Country, r.CustomCIDRs)
		if err != nil || len(cidrs) == 0 {
			continue
		}
		name := geoSetName(r.Country)
		b.WriteString(fmt.Sprintf(
			"        iifname \"%s\" ip saddr @%s drop comment \"geo %s\"\n",
			wan, name, r.Country,
		))
	}
}
