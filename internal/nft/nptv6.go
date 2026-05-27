package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func writeNPTv6Prerouting(b *strings.Builder, nat store.NatState) {
	if !nat.Nptv6Enabled {
		return
	}
	for _, r := range nat.Nptv6Rules {
		internal := strings.TrimSpace(r.InternalPrefix)
		external := strings.TrimSpace(r.ExternalPrefix)
		if internal == "" || external == "" {
			continue
		}
		b.WriteString(fmt.Sprintf(
			"        ip6 daddr %s dnat ip6 prefix to %s\n",
			external, internal,
		))
	}
}

func writeNPTv6Postrouting(b *strings.Builder, cfg Config, nat store.NatState) {
	if !nat.Nptv6Enabled {
		return
	}
	for _, r := range nat.Nptv6Rules {
		internal := strings.TrimSpace(r.InternalPrefix)
		external := strings.TrimSpace(r.ExternalPrefix)
		if internal == "" || external == "" {
			continue
		}
		oif := strings.TrimSpace(r.Oif)
		if oif == "" {
			oif = cfg.DevWAN
		}
		b.WriteString(fmt.Sprintf(
			"        ip6 saddr %s oifname \"%s\" snat ip6 prefix to %s\n",
			internal, oif, external,
		))
	}
}
