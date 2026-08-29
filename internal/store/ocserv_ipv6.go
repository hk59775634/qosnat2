package store

import (
	"fmt"
	"net"
	"strings"

	"github.com/hk59775634/qosnat2/internal/linknet"
)

// FormatOCServIPv6Network 生成 ocserv `ipv6-network` 值（地址或 CIDR）。
// poolPrefix 仅在 network 不含 "/" 时作为池前缀长度；0 时默认 /64。
func FormatOCServIPv6Network(network string, poolPrefix int) string {
	network = strings.TrimSpace(network)
	if network == "" {
		return ""
	}
	if strings.Contains(network, "/") {
		return network
	}
	if poolPrefix <= 0 {
		poolPrefix = 64
	}
	return fmt.Sprintf("%s/%d", network, poolPrefix)
}

// OCServIPv6SubnetPrefix 返回写入 `ipv6-subnet-prefix` 的值；0 表示未启用 IPv6。
func OCServIPv6SubnetPrefix(network string, subnetPrefix int) int {
	if strings.TrimSpace(network) == "" {
		return 0
	}
	if subnetPrefix <= 0 {
		return linknet.OCServDefaultIPv6SubnetPrefix
	}
	return subnetPrefix
}

// ValidateOCServIPv6Pool 校验 IPv6 地址池与每客户端前缀。
// network 为空表示不启用 IPv6（允许）；非空时校验地址族与前缀关系。
func ValidateOCServIPv6Pool(network string, poolPrefix, subnetPrefix int) error {
	network = strings.TrimSpace(network)
	if network == "" {
		if subnetPrefix != 0 {
			return fmt.Errorf("ipv6_subnet_prefix requires ipv6_network")
		}
		return nil
	}
	cidr := FormatOCServIPv6Network(network, poolPrefix)
	ip, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		if ip = net.ParseIP(network); ip == nil || ip.To4() != nil {
			return fmt.Errorf("invalid ipv6_network")
		}
		return fmt.Errorf("invalid ipv6_network cidr")
	}
	if ip.To4() != nil {
		return fmt.Errorf("ipv6_network must be ipv6")
	}
	ones, bits := ipnet.Mask.Size()
	if bits != 128 || ones < 1 || ones > 128 {
		return fmt.Errorf("invalid ipv6_network prefix")
	}
	sub := OCServIPv6SubnetPrefix(network, subnetPrefix)
	if sub < ones || sub > 128 {
		return fmt.Errorf("ipv6_subnet_prefix must be %d–128 (pool is /%d)", ones, ones)
	}
	return nil
}

// OCServHasIPv6 全局/组/vhost 任一配置了 IPv6 池。
func OCServHasIPv6(o OCServState) bool {
	if strings.TrimSpace(o.IPv6Network) != "" {
		return true
	}
	for _, g := range o.Groups {
		if strings.TrimSpace(g.IPv6Network) != "" {
			return true
		}
	}
	for _, v := range o.Vhosts {
		if !v.Enabled {
			continue
		}
		if strings.TrimSpace(v.IPv6Network) != "" {
			return true
		}
	}
	return false
}

// CollectOCServIPv6Pools 收集已启用的 ocserv IPv6 池 CIDR（全局 + 组 + 启用中的 vhost）。
func CollectOCServIPv6Pools(o OCServState) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(network string, poolPrefix int) {
		cidr := FormatOCServIPv6Network(network, poolPrefix)
		if cidr == "" {
			return
		}
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			return
		}
		s := n.String()
		if _, ok := seen[s]; ok {
			return
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	add(o.IPv6Network, 0)
	for _, g := range o.Groups {
		add(g.IPv6Network, 0)
	}
	for _, v := range o.Vhosts {
		if !v.Enabled {
			continue
		}
		add(v.IPv6Network, v.IPv6Prefix)
	}
	return out
}

// ocservIPv6PoolCIDR 返回全局 IPv6 池 CIDR（供 DNS64 access / session limit）。
func ocservIPv6PoolCIDR(o OCServState) string {
	return FormatOCServIPv6Network(o.IPv6Network, 0)
}
