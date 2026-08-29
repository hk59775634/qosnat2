package ocserv

import (
	"fmt"
	"io"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// writeIPv6Pool 写入 ipv6-network + ipv6-subnet-prefix（若已配置）。
func writeIPv6Pool(w io.Writer, network string, poolPrefix, subnetPrefix int) {
	cidr := store.FormatOCServIPv6Network(network, poolPrefix)
	if cidr == "" {
		return
	}
	sub := store.OCServIPv6SubnetPrefix(network, subnetPrefix)
	fmt.Fprintf(w, "ipv6-network = %s\n", cidr)
	if sub > 0 {
		fmt.Fprintf(w, "ipv6-subnet-prefix = %d\n", sub)
	}
}

// appendIPv6DefaultRoute 在已有 IPv6 池且推送了 IPv4 default、但未含 IPv6 默认路由时，补写 ::/0。
func appendIPv6DefaultRoute(routes []string, hasIPv6 bool) []string {
	if !hasIPv6 {
		return routes
	}
	hasV4Default := false
	hasV6Default := false
	for _, r := range routes {
		r = strings.TrimSpace(strings.ToLower(r))
		switch r {
		case "default", "0.0.0.0/0", "0.0.0.0/0.0.0.0":
			hasV4Default = true
		case "::/0", "0::0/0", "default6":
			hasV6Default = true
		}
	}
	if hasV4Default && !hasV6Default {
		return append(append([]string(nil), routes...), "::/0")
	}
	return routes
}
