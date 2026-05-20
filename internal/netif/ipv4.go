package netif

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// LinkExists 检查网卡是否存在
func LinkExists(name string) bool {
	if name == "" {
		return false
	}
	_, err := exec.Command("ip", "link", "show", name).Output()
	return err == nil
}

// PrimaryIPv4 返回网卡上首个 scope global 的 IPv4 地址（不含前缀长度）
func PrimaryIPv4(dev string) (string, error) {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return "", fmt.Errorf("empty device")
	}
	out, err := exec.Command("ip", "-json", "-4", "addr", "show", "dev", dev).Output()
	if err != nil {
		return "", fmt.Errorf("ip addr show %s: %w", dev, err)
	}
	var links []struct {
		AddrInfo []struct {
			Family    string `json:"family"`
			Local     string `json:"local"`
			Scope     string `json:"scope"`
			Prefixlen int    `json:"prefixlen"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(out, &links); err != nil {
		return "", err
	}
	for _, link := range links {
		for _, a := range link.AddrInfo {
			if a.Family != "inet" || a.Local == "" {
				continue
			}
			if a.Scope != "" && a.Scope != "global" {
				continue
			}
			return a.Local, nil
		}
	}
	return "", fmt.Errorf("no global IPv4 on %s", dev)
}
