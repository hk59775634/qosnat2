package nft

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

const sessionLimitSetName = "qosnat2_sess_per_ip"

func writeSessionLimitSet(b *strings.Builder) {
	b.WriteString(fmt.Sprintf("    set %s {\n", sessionLimitSetName))
	b.WriteString("        type ipv4_addr\n")
	b.WriteString("        flags dynamic\n")
	b.WriteString(fmt.Sprintf("        size %d\n", store.SessionLimitSetSize))
	b.WriteString("    }\n\n")
}

func writeSessionLimitRules(b *strings.Builder, st store.State) {
	limit := st.Firewall.MaxSessionsPerIP
	if limit <= 0 {
		return
	}
	cidrs := store.CollectSessionLimitCIDRs(st)
	if len(cidrs) == 0 {
		return
	}
	for _, cidr := range cidrs {
		b.WriteString(fmt.Sprintf(
			"        ct state new ip saddr %s add @%s { ip saddr ct count over %d } drop comment \"qosnat2-per-ip-sess-limit\"\n",
			cidr, sessionLimitSetName, limit,
		))
	}
}
