package nft

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func writeFilterRules(b *strings.Builder, chain string, rules []store.FilterRule) {
	for _, r := range rules {
		if strings.ToLower(r.Chain) != chain {
			continue
		}
		line := r.NftRuleLine()
		if line == "" {
			continue
		}
		b.WriteString("        ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}
