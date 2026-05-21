package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func writeAliasSets(b *strings.Builder, aliases []store.AliasSet) {
	for _, a := range aliases {
		if len(a.Members) == 0 {
			continue
		}
		b.WriteString(fmt.Sprintf("    set %s {\n", a.NftSetName()))
		b.WriteString("        type ipv4_addr\n")
		b.WriteString("        flags interval\n")
		b.WriteString("        elements = { ")
		for i, m := range a.Members {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(m)
		}
		b.WriteString(" }\n")
		b.WriteString("    }\n\n")
	}
}
