package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/lfn"
	"github.com/hk59775634/qosnat2/internal/store"
)

func writeLFNMSS(b *strings.Builder, st store.State) {
	var rules []string
	for _, ic := range st.Network.Ifaces {
		dev := strings.TrimSpace(ic.Device)
		if !ic.LfnEnabled || dev == "" {
			continue
		}
		mss := store.EffectiveLFNMSS(true, ic.LfnMssClamp, lfn.LinkMTU(dev))
		if mss <= 0 {
			continue
		}
		inID := "qosnat2-lfn-mss-" + dev + "-in"
		outID := "qosnat2-lfn-mss-" + dev + "-out"
		rules = append(rules, fmt.Sprintf(
			"        iifname \"%s\" tcp flags syn tcp option maxseg size set %d comment \"%s\"\n",
			dev, mss, inID,
		))
		rules = append(rules, fmt.Sprintf(
			"        oifname \"%s\" tcp flags syn tcp option maxseg size set %d comment \"%s\"\n",
			dev, mss, outID,
		))
	}
	if len(rules) == 0 {
		return
	}
	b.WriteString("    chain lfn_mss {\n")
	b.WriteString("        type filter hook forward priority mangle; policy accept;\n")
	for _, r := range rules {
		b.WriteString(r)
	}
	b.WriteString("    }\n\n")
}
