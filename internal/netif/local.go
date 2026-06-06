package netif

import (
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"strings"
)

// IsAssignedIP 报告 ip 是否为本机地址（含 loopback 或指定网卡上的 global 地址）。
func IsAssignedIP(ip string, devices ...string) bool {
	ip = strings.TrimSpace(ip)
	if ip == "" {
		return false
	}
	if ip == "127.0.0.1" || ip == "::1" {
		return true
	}
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, dev := range devices {
		dev = strings.TrimSpace(dev)
		if dev == "" {
			continue
		}
		if addrs, err := deviceGlobalIPs(dev); err == nil {
			for _, a := range addrs {
				if a == ip {
					return true
				}
			}
		}
	}
	return false
}

func deviceGlobalIPs(dev string) ([]string, error) {
	family := "-4"
	out, err := exec.Command("ip", "-json", family, "addr", "show", "dev", dev).Output()
	if err != nil {
		return nil, fmt.Errorf("ip addr show %s: %w", dev, err)
	}
	var links []struct {
		AddrInfo []struct {
			Local string `json:"local"`
			Scope string `json:"scope"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(out, &links); err != nil {
		return nil, err
	}
	var ips []string
	for _, link := range links {
		for _, a := range link.AddrInfo {
			if a.Local == "" {
				continue
			}
			if a.Scope != "" && a.Scope != "global" {
				continue
			}
			ips = append(ips, a.Local)
		}
	}
	out6, err := exec.Command("ip", "-json", "-6", "addr", "show", "dev", dev).Output()
	if err == nil {
		var links6 []struct {
			AddrInfo []struct {
				Local string `json:"local"`
				Scope string `json:"scope"`
			} `json:"addr_info"`
		}
		if json.Unmarshal(out6, &links6) == nil {
			for _, link := range links6 {
				for _, a := range link.AddrInfo {
					if a.Local == "" {
						continue
					}
					if a.Scope != "" && a.Scope != "global" {
						continue
					}
					ips = append(ips, a.Local)
				}
			}
		}
	}
	return ips, nil
}
