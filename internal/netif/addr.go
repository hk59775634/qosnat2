package netif

import (
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// SetConfig 更新网卡 IPv4 地址与 up/down 状态
func SetConfig(dev string, ipv4 []string, up *bool) error {
	dev = strings.TrimSpace(dev)
	if dev == "" || dev == "lo" {
		return fmt.Errorf("invalid device")
	}
	if !LinkExists(dev) {
		return fmt.Errorf("interface %q not found", dev)
	}
	if ipv4 != nil {
		if out, err := exec.Command("ip", "addr", "flush", "dev", dev, "proto", "inet").CombinedOutput(); err != nil {
			return fmt.Errorf("flush %s: %s %w", dev, strings.TrimSpace(string(out)), err)
		}
		for _, cidr := range ipv4 {
			cidr = strings.TrimSpace(cidr)
			if cidr == "" {
				continue
			}
			if _, _, err := net.ParseCIDR(cidr); err != nil {
				if ip := net.ParseIP(cidr); ip == nil || ip.To4() == nil {
					return fmt.Errorf("invalid ipv4 cidr: %q", cidr)
				}
				cidr += "/32"
			}
			if out, err := exec.Command("ip", "addr", "add", cidr, "dev", dev).CombinedOutput(); err != nil {
				msg := strings.TrimSpace(string(out))
				if !strings.Contains(msg, "File exists") {
					return fmt.Errorf("add %s to %s: %s %w", cidr, dev, msg, err)
				}
			}
		}
	}
	if up != nil {
		state := "down"
		if *up {
			state = "up"
		}
		if out, err := exec.Command("ip", "link", "set", dev, state).CombinedOutput(); err != nil {
			return fmt.Errorf("link set %s %s: %s %w", dev, state, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}
