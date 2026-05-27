package netif

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// PrimaryIPv6 返回网卡上首个 scope global 的 IPv6 地址（不含前缀长度）
func PrimaryIPv6(dev string) (string, error) {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return "", fmt.Errorf("empty device")
	}
	out, err := execCommandJSON(dev)
	if err != nil {
		return "", err
	}
	for _, link := range out {
		for _, a := range link.AddrInfo {
			if a.Family != "inet6" || a.Local == "" {
				continue
			}
			if strings.HasPrefix(strings.ToLower(a.Local), "fe80:") {
				continue
			}
			if a.Scope != "" && a.Scope != "global" {
				continue
			}
			return a.Local, nil
		}
	}
	return "", fmt.Errorf("no global IPv6 on %s", dev)
}

type addrLink struct {
	AddrInfo []struct {
		Family    string `json:"family"`
		Local     string `json:"local"`
		Scope     string `json:"scope"`
		Prefixlen int    `json:"prefixlen"`
	} `json:"addr_info"`
}

func execCommandJSON(dev string) ([]addrLink, error) {
	out, err := exec.Command("ip", "-json", "-6", "addr", "show", "dev", dev).Output()
	if err != nil {
		return nil, fmt.Errorf("ip addr show %s: %w", dev, err)
	}
	var links []addrLink
	if err := json.Unmarshal(out, &links); err != nil {
		return nil, err
	}
	return links, nil
}
