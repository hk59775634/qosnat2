package store

import (
	"fmt"
	"regexp"
	"strings"
)

var ocservGroupNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// OCServGroup 用户组；Apply 时写入 config-per-group/<name>；未 OmitSelectGroup 时另写 select-group
type OCServGroup struct {
	Name            string   `json:"name"`
	Label           string   `json:"label,omitempty"` // select-group 展示名 [label]
	Comment         string   `json:"comment,omitempty"`
	OmitSelectGroup bool     `json:"omit_select_group,omitempty"` // true: 仅 config-per-group，不写入 ocserv.conf select-group
	DNS          []string `json:"dns,omitempty"`
	Routes       []string `json:"routes,omitempty"`
	NoRoutes     []string `json:"no_routes,omitempty"`
	IPv4Network  string   `json:"ipv4_network,omitempty"`
	IPv4Netmask  string   `json:"ipv4_netmask,omitempty"`
	RxDataPerSec int      `json:"rx_data_per_sec,omitempty"`
	TxDataPerSec int      `json:"tx_data_per_sec,omitempty"`
	MTU          int      `json:"mtu,omitempty"`
	TunnelAllDNS bool     `json:"tunnel_all_dns,omitempty"`
}

func normalizeOCServGroupName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("group name required")
	}
	if !ocservGroupNameRe.MatchString(name) {
		return "", fmt.Errorf("invalid group name (use letters, digits, ._-)")
	}
	return name, nil
}

func normalizeOCServVhostDomain(d string) (string, error) {
	d = strings.TrimSpace(strings.ToLower(d))
	if d == "" {
		return "", fmt.Errorf("vhost domain required")
	}
	if strings.ContainsAny(d, " /\\") {
		return "", fmt.Errorf("invalid vhost domain")
	}
	return d, nil
}

// NormalizeOCServGroups 校验并规范化组列表
func NormalizeOCServGroups(groups *[]OCServGroup) error {
	if groups == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]OCServGroup, 0, len(*groups))
	for i := range *groups {
		n, err := normalizeOCServGroupName((*groups)[i].Name)
		if err != nil {
			return err
		}
		if seen[n] {
			return fmt.Errorf("duplicate group %s", n)
		}
		seen[n] = true
		g := (*groups)[i]
		g.Name = n
		g.Label = strings.TrimSpace(g.Label)
		g.DNS = trimStringList(g.DNS)
		g.Routes = trimStringList(g.Routes)
		g.NoRoutes = trimStringList(g.NoRoutes)
		out = append(out, g)
	}
	*groups = out
	return nil
}

// NormalizeOCServVhosts 校验并规范化 vhost 列表（enabled=false 的 vhost 保留在 state，不写入 ocserv.conf）
func NormalizeOCServVhosts(vhosts *[]OCServVhost, authMethod string) error {
	if vhosts == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]OCServVhost, 0, len(*vhosts))
	for i := range *vhosts {
		v := (*vhosts)[i]
		if !v.Enabled {
			d, err := normalizeOCServVhostDomain(v.Domain)
			if err != nil {
				return err
			}
			if seen[d] {
				return fmt.Errorf("duplicate vhost %s", d)
			}
			seen[d] = true
			v.Domain = d
			v.Enabled = false
			if v.Users == nil {
				v.Users = []OCServUser{}
			}
			out = append(out, v)
			continue
		}
		d, err := normalizeOCServVhostDomain(v.Domain)
		if err != nil {
			return err
		}
		if seen[d] {
			return fmt.Errorf("duplicate vhost %s", d)
		}
		seen[d] = true
		v.Domain = d
		am := strings.TrimSpace(v.AuthMethod)
		if am != "" && am != OCServAuthPlain && am != OCServAuthRadius && am != "certificate" {
			return fmt.Errorf("vhost auth must be plain, radius, certificate or empty")
		}
		if am == OCServAuthRadius && !VhostUsesOwnRadius(v) && authMethod != OCServAuthRadius {
			return fmt.Errorf("vhost %s: radius auth requires global RADIUS or per-vhost radius.server", d)
		}
		if VhostUsesOwnRadius(v) {
			if err := normalizeVhostRadius(v.Radius); err != nil {
				return fmt.Errorf("vhost %s: %w", d, err)
			}
		}
		if am == OCServAuthPlain && strings.TrimSpace(v.PlainPasswdPath) == "" {
			v.PlainPasswdPath = ""
		}
		rm := strings.TrimSpace(v.RekeyMethod)
		if rm != "" && rm != "ssl" && rm != "new-tunnel" {
			return fmt.Errorf("vhost %s: rekey_method must be ssl or new-tunnel", d)
		}
		v.AuthMethod = am
		v.DNS = trimStringList(v.DNS)
		v.NBNS = trimStringList(v.NBNS)
		v.Routes = trimStringList(v.Routes)
		v.NoRoutes = trimStringList(v.NoRoutes)
		v.IRoutes = trimStringList(v.IRoutes)
		v.SelectGroups = trimStringList(v.SelectGroups)
		if v.Users == nil {
			v.Users = []OCServUser{}
		}
		for j := range v.Users {
			u := strings.TrimSpace(v.Users[j].Username)
			if u == "" {
				return fmt.Errorf("vhost %s: user username required", d)
			}
			v.Users[j].Username = u
			v.Users[j].Group = strings.TrimSpace(v.Users[j].Group)
			v.Users[j].Comment = strings.TrimSpace(v.Users[j].Comment)
		}
		out = append(out, v)
	}
	*vhosts = out
	return nil
}
