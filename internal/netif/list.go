package netif

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

// Addr 地址条目
type Addr struct {
	Family string `json:"family"`
	CIDR   string `json:"cidr"`
	Scope  string `json:"scope,omitempty"`
}

// Detail 网卡详情
type Detail struct {
	Name      string `json:"name"`
	Up        bool   `json:"up"`
	OperState string `json:"operstate"`
	MAC       string `json:"mac,omitempty"`
	Addrs     []Addr `json:"addrs"`
}

// ListDetails 列出宿主机网卡（不含 lo）
func ListDetails() ([]Detail, error) {
	out, err := exec.Command("ip", "-json", "addr", "show").Output()
	if err != nil {
		return nil, fmt.Errorf("ip addr: %w", err)
	}
	var raw []struct {
		Ifname    string `json:"ifname"`
		OperState string `json:"operstate"`
		Address   string `json:"address"`
		AddrInfo  []struct {
			Family    string `json:"family"`
			Local     string `json:"local"`
			Prefixlen int    `json:"prefixlen"`
			Scope     string `json:"scope"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}
	var list []Detail
	for _, l := range raw {
		if l.Ifname == "lo" {
			continue
		}
		d := Detail{
			Name:      l.Ifname,
			OperState: l.OperState,
			Up:        l.OperState == "UP" || l.OperState == "UNKNOWN",
			MAC:       l.Address,
		}
		for _, a := range l.AddrInfo {
			if a.Family != "inet" && a.Family != "inet6" {
				continue
			}
			if a.Local == "" {
				continue
			}
			d.Addrs = append(d.Addrs, Addr{
				Family: a.Family,
				CIDR:   fmt.Sprintf("%s/%d", a.Local, a.Prefixlen),
				Scope:  a.Scope,
			})
		}
		list = append(list, d)
	}
	return list, nil
}

// IPv4Addrs 返回网卡上所有 IPv4 CIDR
func IPv4Addrs(dev string) ([]string, error) {
	list, err := ListDetails()
	if err != nil {
		return nil, err
	}
	for _, d := range list {
		if d.Name != dev {
			continue
		}
		var out []string
		for _, a := range d.Addrs {
			if a.Family == "inet" {
				out = append(out, a.CIDR)
			}
		}
		return out, nil
	}
	return nil, fmt.Errorf("interface %q not found", dev)
}
