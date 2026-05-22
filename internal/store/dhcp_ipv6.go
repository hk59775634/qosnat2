package store

import (
	"fmt"
	"net"
	"strings"
)

// NormalizeDHCPv6 校验 IPv6 前缀与池（在 NormalizeDHCP 末尾调用）
func NormalizeDHCPv6(d *DHCPState) error {
	if d == nil || !d.Enabled {
		return nil
	}
	if !d.IPv6Enabled && !d.RAEnabled {
		d.IPv6Prefix = ""
		d.IPv6Start = ""
		d.IPv6End = ""
		return nil
	}
	prefix := strings.TrimSpace(d.IPv6Prefix)
	if d.IPv6Enabled {
		if prefix == "" {
			return fmt.Errorf("ipv6_prefix required when ipv6_enabled")
		}
		_, ipNet, err := net.ParseCIDR(prefix)
		if err != nil || ipNet == nil || len(ipNet.IP) != net.IPv6len {
			return fmt.Errorf("invalid ipv6_prefix: %q", prefix)
		}
		ones, _ := ipNet.Mask.Size()
		if ones < 48 || ones > 64 {
			return fmt.Errorf("ipv6_prefix length should be /48-/64 for dhcp-range")
		}
		start := strings.TrimSpace(d.IPv6Start)
		end := strings.TrimSpace(d.IPv6End)
		if start == "" || end == "" {
			return fmt.Errorf("ipv6_start and ipv6_end required when ipv6_enabled")
		}
		if ip := net.ParseIP(start); ip == nil || ip.To16() == nil {
			return fmt.Errorf("invalid ipv6_start: %q", start)
		}
		if ip := net.ParseIP(end); ip == nil || ip.To16() == nil {
			return fmt.Errorf("invalid ipv6_end: %q", end)
		}
		d.IPv6Prefix = prefix
		d.IPv6Start = start
		d.IPv6End = end
	} else if d.RAEnabled {
		if prefix == "" {
			return fmt.Errorf("ipv6_prefix required when ra_enabled")
		}
		if _, n, err := net.ParseCIDR(prefix); err != nil || n == nil {
			return fmt.Errorf("invalid ipv6_prefix: %q", prefix)
		}
		d.IPv6Prefix = prefix
	}
	if d.RAEnabled && d.RAIntervalSec < 0 {
		d.RAIntervalSec = 0
	}
	return nil
}
