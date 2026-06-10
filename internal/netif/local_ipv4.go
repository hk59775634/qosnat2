package netif

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"sort"
)

// CollectLocalGlobalIPv4 返回本机所有网卡上的 scope global IPv4（去重排序）。
func CollectLocalGlobalIPv4() ([]string, error) {
	out, err := exec.Command("ip", "-json", "-4", "addr", "show").Output()
	if err != nil {
		return nil, fmt.Errorf("ip -4 addr show: %w", err)
	}
	var links []struct {
		AddrInfo []struct {
			Family string `json:"family"`
			Local  string `json:"local"`
			Scope  string `json:"scope"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(out, &links); err != nil {
		return nil, err
	}
	seen := map[string]struct{}{}
	for _, link := range links {
		for _, a := range link.AddrInfo {
			if a.Family != "inet" || a.Local == "" {
				continue
			}
			if a.Scope != "" && a.Scope != "global" {
				continue
			}
			seen[a.Local] = struct{}{}
		}
	}
	ips := make([]string, 0, len(seen))
	for ip := range seen {
		ips = append(ips, ip)
	}
	sort.Strings(ips)
	return ips, nil
}
