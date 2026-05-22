package store

import (
	"fmt"
	"net"
	"regexp"
	"strings"
)

// DHCPStaticLease 静态 DHCP 绑定
type DHCPStaticLease struct {
	MAC      string `json:"mac"`
	IP       string `json:"ip"`
	Hostname string `json:"hostname,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// DHCPState LAN DHCP 服务（由 dnsmasq 托管）
type DHCPState struct {
	Enabled        bool              `json:"enabled"`
	Interface      string            `json:"interface"`       // 监听网卡，空则使用 DEV_LAN
	RangeStart     string            `json:"range_start"`     // 池起始，如 192.168.1.100
	RangeEnd       string            `json:"range_end"`       // 池结束
	Router         string            `json:"router"`          // option 3 默认网关
	Netmask        string            `json:"netmask,omitempty"` // 如 255.255.255.0；空则 255.255.255.0
	Domain         string            `json:"domain,omitempty"`
	DNSServers     []string          `json:"dns_servers"`
	LeaseTimeSec   int               `json:"lease_time_sec"`  // 默认 86400
	Authoritative  bool              `json:"authoritative"`
	StaticLeases   []DHCPStaticLease `json:"static_leases"`
	IPv6Enabled    bool              `json:"ipv6_enabled"`
	IPv6Prefix     string            `json:"ipv6_prefix,omitempty"`
	IPv6Start      string            `json:"ipv6_start,omitempty"`
	IPv6End        string            `json:"ipv6_end,omitempty"`
	RAEnabled      bool              `json:"ra_enabled"`
	RAIntervalSec  int               `json:"ra_interval_sec,omitempty"`
}

var macRE = regexp.MustCompile(`^([0-9a-f]{2}:){5}[0-9a-f]{2}$`)

// NormalizeMAC 小写冒号分隔
func NormalizeMAC(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.ReplaceAll(s, "-", ":")
	if !macRE.MatchString(s) {
		return "", fmt.Errorf("invalid mac: %q", s)
	}
	return s, nil
}

// NormalizeDHCP 校验并填充默认值
func NormalizeDHCP(d *DHCPState, defaultIface string) error {
	if d == nil {
		return fmt.Errorf("dhcp config nil")
	}
	iface := strings.TrimSpace(d.Interface)
	if iface == "" {
		iface = strings.TrimSpace(defaultIface)
	}
	d.Interface = iface
	if !d.Enabled {
		return nil
	}
	if iface == "" {
		return fmt.Errorf("interface required when dhcp enabled")
	}
	rs := strings.TrimSpace(d.RangeStart)
	re := strings.TrimSpace(d.RangeEnd)
	if rs == "" || re == "" {
		return fmt.Errorf("range_start and range_end required")
	}
	if net.ParseIP(rs) == nil || net.ParseIP(re) == nil {
		return fmt.Errorf("invalid dhcp range ip")
	}
	if strings.TrimSpace(d.Router) == "" {
		return fmt.Errorf("router (default gateway) required")
	}
	if net.ParseIP(d.Router) == nil {
		return fmt.Errorf("invalid router ip")
	}
	if d.LeaseTimeSec <= 0 {
		d.LeaseTimeSec = 86400
	}
	if d.Netmask == "" {
		d.Netmask = "255.255.255.0"
	}
	if d.DNSServers == nil {
		d.DNSServers = []string{}
	}
	var clean []string
	for _, s := range d.DNSServers {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if net.ParseIP(s) == nil {
			return fmt.Errorf("invalid dns server: %q", s)
		}
		clean = append(clean, s)
	}
	d.DNSServers = clean
	if d.StaticLeases == nil {
		d.StaticLeases = []DHCPStaticLease{}
	}
	var leases []DHCPStaticLease
	for _, sl := range d.StaticLeases {
		mac, err := NormalizeMAC(sl.MAC)
		if err != nil {
			return err
		}
		ip := strings.TrimSpace(sl.IP)
		if net.ParseIP(ip) == nil {
			return fmt.Errorf("invalid static lease ip: %q", sl.IP)
		}
		leases = append(leases, DHCPStaticLease{
			MAC:      mac,
			IP:       ip,
			Hostname: strings.TrimSpace(sl.Hostname),
			Comment:  strings.TrimSpace(sl.Comment),
		})
	}
	d.StaticLeases = leases
	return NormalizeDHCPv6(d)
}

// DefaultDHCP 默认配置（未启用）
func DefaultDHCP() DHCPState {
	return DHCPState{
		Enabled:       false,
		RangeStart:    "192.168.1.100",
		RangeEnd:      "192.168.1.254",
		Router:        "192.168.1.1",
		Netmask:       "255.255.255.0",
		DNSServers:    []string{"8.8.8.8", "1.1.1.1"},
		LeaseTimeSec:  86400,
		Authoritative: true,
		StaticLeases:  []DHCPStaticLease{},
	}
}
