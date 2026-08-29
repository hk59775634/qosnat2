package nft

import (
	"fmt"
	"net"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// writeOCServIPv6Masquerade 为 ocserv IPv6 池做 WAN masquerade。
// 若 NPTv6 已覆盖相同 internal 前缀则跳过（避免双重翻译）。
func writeOCServIPv6Masquerade(b *strings.Builder, cfg Config, st store.State) {
	o := st.VPN.OCServ
	if !o.Enabled {
		return
	}
	pools := store.CollectOCServIPv6Pools(o)
	if len(pools) == 0 {
		return
	}
	nptCovered := map[string]struct{}{}
	if st.Nat.Nptv6Enabled {
		for _, r := range st.Nat.Nptv6Rules {
			if c := normalizeIPv6CIDR(r.InternalPrefix); c != "" {
				nptCovered[c] = struct{}{}
			}
		}
	}
	wan := strings.TrimSpace(cfg.DevWAN)
	if wan == "" {
		return
	}
	for _, pool := range pools {
		if _, ok := nptCovered[pool]; ok {
			continue
		}
		b.WriteString(fmt.Sprintf(
			"        ip6 saddr %s oifname \"%s\" masquerade comment \"qosnat2-ocserv-ipv6\"\n",
			pool, wan,
		))
	}
}

func normalizeIPv6CIDR(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return ""
	}
	if n.IP.To4() != nil {
		return ""
	}
	return n.String()
}
