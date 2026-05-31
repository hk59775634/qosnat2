package store

import (
	"fmt"
	"net"
	"strings"
)

// ValidateCIDR 校验 IPv4 CIDR，拒绝换行/引号等可破坏 nft 配置的字符。
func ValidateCIDR(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("empty cidr")
	}
	if strings.ContainsAny(s, "\n\r\t\"'\\") {
		return fmt.Errorf("invalid characters in cidr")
	}
	if _, _, err := net.ParseCIDR(s); err != nil {
		return fmt.Errorf("invalid cidr: %w", err)
	}
	return nil
}

// ValidateIPv4OrCIDR 校验 IPv4 地址或 CIDR。
func ValidateIPv4OrCIDR(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("empty")
	}
	if strings.ContainsAny(s, "\n\r\t\"'\\") {
		return fmt.Errorf("invalid characters")
	}
	if strings.Contains(s, "/") {
		ip, n, err := net.ParseCIDR(s)
		if err != nil {
			return fmt.Errorf("invalid cidr: %w", err)
		}
		if ip.To4() == nil {
			return fmt.Errorf("invalid ipv4 cidr: %s", s)
		}
		_ = n
		return nil
	}
	if ip := net.ParseIP(s); ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid ipv4: %s", s)
	}
	return nil
}

// ValidateIPv6OrCIDR 校验 IPv6 地址或 CIDR。
func ValidateIPv6OrCIDR(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("empty")
	}
	if strings.ContainsAny(s, "\n\r\t\"'\\") {
		return fmt.Errorf("invalid characters")
	}
	if strings.Contains(s, "/") {
		ip, _, err := net.ParseCIDR(s)
		if err != nil {
			return fmt.Errorf("invalid cidr: %w", err)
		}
		if ip.To4() != nil {
			return fmt.Errorf("invalid ipv6 cidr: %s", s)
		}
		return nil
	}
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() != nil {
		return fmt.Errorf("invalid ipv6: %s", s)
	}
	return nil
}

// IPInCIDR 判断 IPv4 是否落在 cidr 内
func IPInCIDR(ip, cidr string) bool {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil {
		return false
	}
	p := net.ParseIP(ip)
	if p == nil {
		return false
	}
	return n.Contains(p)
}

// ProfileByCIDR 在列表中查找 profile
func ProfileByCIDR(profiles []ProfileEntry, cidr string) (ProfileEntry, bool) {
	for _, p := range profiles {
		if p.CIDR == cidr {
			return p, true
		}
	}
	return ProfileEntry{}, false
}
