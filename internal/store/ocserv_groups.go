package store

import (
	"fmt"
	"regexp"
	"strings"
)

var ocservGroupNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// OCServGroup 用户组；Apply 时写入 config-per-group/<name>.conf 与 select-group
type OCServGroup struct {
	Name         string   `json:"name"`
	Label        string   `json:"label,omitempty"` // select-group 展示名 [label]
	Comment      string   `json:"comment,omitempty"`
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

// OCServVhost 虚拟主机 [vhost:domain] 段
type OCServVhost struct {
	Enabled        bool     `json:"enabled"`
	Domain         string   `json:"domain"`
	AuthMethod     string   `json:"auth_method,omitempty"` // 空=继承全局；plain|radius|certificate
	ServerCertPath string   `json:"server_cert_path,omitempty"`
	ServerKeyPath  string   `json:"server_key_path,omitempty"`
	CaCertPath     string   `json:"ca_cert_path,omitempty"`
	IPv4Network    string   `json:"ipv4_network,omitempty"`
	IPv4Netmask    string   `json:"ipv4_netmask,omitempty"`
	DNS            []string `json:"dns,omitempty"`
	Routes         []string `json:"routes,omitempty"`
	CertUserOID    string   `json:"cert_user_oid,omitempty"`
	Comment        string   `json:"comment,omitempty"`
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

// NormalizeOCServVhosts 校验并规范化 vhost 列表
func NormalizeOCServVhosts(vhosts *[]OCServVhost, authMethod string) error {
	if vhosts == nil {
		return nil
	}
	seen := map[string]bool{}
	out := make([]OCServVhost, 0, len(*vhosts))
	for i := range *vhosts {
		if !(*vhosts)[i].Enabled {
			continue
		}
		d, err := normalizeOCServVhostDomain((*vhosts)[i].Domain)
		if err != nil {
			return err
		}
		if seen[d] {
			return fmt.Errorf("duplicate vhost %s", d)
		}
		seen[d] = true
		v := (*vhosts)[i]
		v.Domain = d
		am := strings.TrimSpace(v.AuthMethod)
		if am != "" && am != OCServAuthPlain && am != OCServAuthRadius && am != "certificate" {
			return fmt.Errorf("vhost auth must be plain, radius, certificate or empty")
		}
		if am == OCServAuthRadius && authMethod != OCServAuthRadius {
			return fmt.Errorf("vhost %s: radius auth requires global RADIUS", d)
		}
		v.AuthMethod = am
		v.DNS = trimStringList(v.DNS)
		v.Routes = trimStringList(v.Routes)
		out = append(out, v)
	}
	*vhosts = out
	return nil
}
