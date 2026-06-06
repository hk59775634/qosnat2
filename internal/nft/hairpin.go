package nft

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

// HairpinAddrResolver 供 SyncAutoFilterRules 解析 WAN 公网地址与本机地址。
func HairpinAddrResolver(devLAN, devWAN string) store.HairpinAddrResolver {
	var devs []string
	for _, d := range []string{devLAN, devWAN} {
		if d = strings.TrimSpace(d); d != "" {
			devs = append(devs, d)
		}
	}
	return store.HairpinAddrResolver{
		PrimaryIPv4: netif.PrimaryIPv4,
		PrimaryIPv6: netif.PrimaryIPv6,
		IsLocalIP: func(ip string) bool {
			return netif.IsAssignedIP(ip, devs...)
		},
	}
}
