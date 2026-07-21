package nft

import (
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

func writeFilterRules(b *strings.Builder, chain string, rules []store.FilterRule, schedules []store.Schedule) {
	now := time.Now()
	for _, r := range rules {
		if strings.ToLower(r.Chain) != chain {
			continue
		}
		line := r.NftRuleLineAt(schedules, now)
		if line == "" {
			continue
		}
		b.WriteString("        ")
		b.WriteString(line)
		b.WriteString("\n")
	}
}
