package store

import (
	"fmt"
	"net"
	"strings"
)

const (
	DNS64ModeLocal    = "local_unbound"
	DNS64ModeUpstream = "upstream"

	DefaultNat64Prefix = "64:ff9b::/96"
	DefaultNat64Pool4  = "10.255.255.0/24"

	GoogleDNS64Primary   = "2001:4860:4860::64"
	GoogleDNS64Secondary = "2001:4860:4860::6464"
)

// NatIPv4State IPv4 出站 SNAT / 策略路由
type NatIPv4State struct {
	// Enabled 出站 IPv4 NAT 总开关；nil/省略视为 true（兼容旧 state）。
	Enabled        *bool             `json:"enabled,omitempty"`
	PolicyRoutes   []string          `json:"policy_routes"`
	SharedIPs      []string          `json:"shared_ips"`
	StaticMappings map[string]string `json:"static_mappings"`
	PrefixMappings map[string]string `json:"prefix_mappings"`
}

// NatIPv4Enabled 是否应用 IPv4 出站 NAT（策略网段、共享 IP、1:1、masquerade 等）。
func NatIPv4Enabled(n NatIPv4State) bool {
	if n.Enabled == nil {
		return true
	}
	return *n.Enabled
}

// Nptv6Rule RFC 6296 无状态前缀映射
type Nptv6Rule struct {
	ID             string `json:"id"`
	InternalPrefix string `json:"internal_prefix"`
	ExternalPrefix string `json:"external_prefix"`
	Description    string `json:"description,omitempty"`
	Oif            string `json:"oif,omitempty"`
}

// DNS64Config DNS64 解析（Unbound 本地或公网上游）
type DNS64Config struct {
	Mode           string   `json:"mode"`
	Upstream       []string `json:"upstream,omitempty"`
	UnboundListen  string   `json:"unbound_listen,omitempty"` // relay: 127.0.0.1:5353；直连: 网关:53 或 0.0.0.0:53
	Forwarders     []string `json:"forwarders,omitempty"`
	ServeToClients bool     `json:"serve_to_clients"`         // true=经 dnsmasq/DHCP 下发；false=VPN/静态 DNS（直连 Unbound 或公网 DNS64）
	AccessAllow    []string `json:"access_allow,omitempty"`   // Unbound 直连时允许的查询源 CIDR
}

// NatState NAT / NPTv6 / NAT64 / DNS64
type NatState struct {
	IPv4         NatIPv4State `json:"ipv4"`
	Nptv6Enabled bool         `json:"nptv6_enabled"`
	Nptv6Rules   []Nptv6Rule  `json:"nptv6_rules,omitempty"`
	Nat64Enabled bool         `json:"nat64_enabled"`
	Nat64Prefix  string       `json:"nat64_prefix,omitempty"`
	Nat64Pool4   string       `json:"nat64_pool4,omitempty"`
	DNS64        DNS64Config  `json:"dns64"`
}

// DefaultNat 默认 NAT 配置
func DefaultNat() NatState {
	enabled := true
	return NatState{
		IPv4: NatIPv4State{
			Enabled:        &enabled,
			PolicyRoutes:   []string{},
			SharedIPs:      nil,
			StaticMappings: map[string]string{},
			PrefixMappings: map[string]string{},
		},
		Nat64Prefix: DefaultNat64Prefix,
		Nat64Pool4:  DefaultNat64Pool4,
		DNS64: DNS64Config{
			Mode:           DNS64ModeLocal,
			UnboundListen:  "127.0.0.1:5353",
			Forwarders:     []string{"1.1.1.1", "8.8.8.8"},
			Upstream:       []string{GoogleDNS64Primary, GoogleDNS64Secondary},
			ServeToClients: true,
		},
	}
}

// natLegacyFields 旧版 state.json 顶层 IPv4 NAT 字段（仅加载迁移）
type natLegacyFields struct {
	PolicyRoutes   []string          `json:"policy_routes"`
	SharedIPs      []string          `json:"shared_ips"`
	StaticMappings map[string]string `json:"static_mappings"`
	PrefixMappings map[string]string `json:"prefix_mappings"`
}

// MigrateNatFromLegacy 将旧顶层字段迁入 nat.ipv4（按字段合并，避免部分迁移遗漏）
func MigrateNatFromLegacy(st *State, leg natLegacyFields) {
	if len(leg.PolicyRoutes) > 0 && len(st.Nat.IPv4.PolicyRoutes) == 0 {
		st.Nat.IPv4.PolicyRoutes = append([]string(nil), leg.PolicyRoutes...)
	}
	if len(leg.SharedIPs) > 0 && len(st.Nat.IPv4.SharedIPs) == 0 {
		st.Nat.IPv4.SharedIPs = append([]string(nil), leg.SharedIPs...)
	}
	if len(leg.StaticMappings) > 0 && len(st.Nat.IPv4.StaticMappings) == 0 {
		st.Nat.IPv4.StaticMappings = leg.StaticMappings
	}
	if len(leg.PrefixMappings) > 0 && len(st.Nat.IPv4.PrefixMappings) == 0 {
		st.Nat.IPv4.PrefixMappings = leg.PrefixMappings
	}
}

func ensureNatDefaults(n *NatState) {
	def := DefaultNat()
	if n.IPv4.PolicyRoutes == nil {
		n.IPv4.PolicyRoutes = []string{}
	}
	// 允许空 policy_routes（纯三层 / 仅 oif masquerade）；勿再回填默认 10.0.0.0/8。
	if n.IPv4.SharedIPs == nil {
		n.IPv4.SharedIPs = []string{}
	}
	if n.IPv4.StaticMappings == nil {
		n.IPv4.StaticMappings = map[string]string{}
	}
	if n.IPv4.PrefixMappings == nil {
		n.IPv4.PrefixMappings = map[string]string{}
	}
	if strings.TrimSpace(n.Nat64Prefix) == "" {
		n.Nat64Prefix = DefaultNat64Prefix
	}
	if strings.TrimSpace(n.Nat64Pool4) == "" {
		n.Nat64Pool4 = DefaultNat64Pool4
	}
	if strings.TrimSpace(n.DNS64.Mode) == "" {
		n.DNS64.Mode = DNS64ModeLocal
	}
	if n.DNS64.ServeToClients && strings.TrimSpace(n.DNS64.UnboundListen) == "" {
		n.DNS64.UnboundListen = def.DNS64.UnboundListen
	}
	if len(n.DNS64.Forwarders) == 0 {
		n.DNS64.Forwarders = append([]string(nil), def.DNS64.Forwarders...)
	}
	if len(n.DNS64.Upstream) == 0 {
		n.DNS64.Upstream = append([]string(nil), def.DNS64.Upstream...)
	}
	MigrateNptv6RuleIDs(&n.Nptv6Rules)
	EnsureDNS64Defaults(&n.DNS64)
}

// EnsureDNS64Defaults 补全 DNS64 子配置
func EnsureDNS64Defaults(d *DNS64Config) {
	def := DefaultNat().DNS64
	if strings.TrimSpace(d.Mode) == "" {
		d.Mode = def.Mode
	}
	// VPN/直连模式留空，由 EffectiveUnboundListen 解析为网关 :53
	if d.ServeToClients && strings.TrimSpace(d.UnboundListen) == "" {
		d.UnboundListen = def.UnboundListen
	}
	if len(d.Forwarders) == 0 {
		d.Forwarders = append([]string(nil), def.Forwarders...)
	}
	if len(d.Upstream) == 0 {
		d.Upstream = append([]string(nil), def.Upstream...)
	}
}

// MigrateNptv6RuleIDs 为无 id 的规则生成稳定 id
func MigrateNptv6RuleIDs(rules *[]Nptv6Rule) {
	for i := range *rules {
		r := &(*rules)[i]
		if strings.TrimSpace(r.ID) == "" {
			r.ID = fmt.Sprintf("nptv6-%d", i+1)
		}
	}
}

// ValidateNptv6Rule 校验 NPTv6 规则
func ValidateNptv6Rule(r Nptv6Rule) error {
	inner, err := parseIPv6CIDR(r.InternalPrefix)
	if err != nil {
		return fmt.Errorf("internal_prefix: %w", err)
	}
	outer, err := parseIPv6CIDR(r.ExternalPrefix)
	if err != nil {
		return fmt.Errorf("external_prefix: %w", err)
	}
	if inner.ones != outer.ones {
		return fmt.Errorf("prefix lengths must match (%d vs %d)", inner.ones, outer.ones)
	}
	return nil
}

// ValidateDNS64Config 校验 DNS64 模式（与 NAT64 开关无关）
func ValidateDNS64Config(d DNS64Config) error {
	switch d.Mode {
	case "", DNS64ModeLocal, DNS64ModeUpstream:
	default:
		return fmt.Errorf("dns64.mode must be %q or %q", DNS64ModeLocal, DNS64ModeUpstream)
	}
	if d.Mode == DNS64ModeUpstream {
		for _, u := range d.Upstream {
			if err := validateIPv6Addr(u); err != nil {
				return fmt.Errorf("dns64.upstream: %w", err)
			}
		}
	}
	return nil
}

// ValidateNat64Config 校验 NAT64 + DNS64
func ValidateNat64Config(n NatState) error {
	if err := ValidateDNS64Config(n.DNS64); err != nil {
		return err
	}
	if !n.Nat64Enabled {
		return nil
	}
	pfx, err := parseIPv6CIDR(n.Nat64Prefix)
	if err != nil {
		return fmt.Errorf("nat64_prefix: %w", err)
	}
	if pfx.ones != 96 {
		return fmt.Errorf("nat64_prefix must be /96 (got /%d)", pfx.ones)
	}
	if err := ValidateIPv4OrCIDR(n.Nat64Pool4); err != nil {
		return fmt.Errorf("nat64_pool4: %w", err)
	}
	if n.DNS64.Mode == DNS64ModeUpstream && len(n.DNS64.Upstream) == 0 {
		return fmt.Errorf("dns64.upstream required for upstream mode")
	}
	return nil
}

type parsedV6CIDR struct {
	ones int
}

func parseIPv6CIDR(s string) (parsedV6CIDR, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return parsedV6CIDR{}, fmt.Errorf("empty cidr")
	}
	if strings.ContainsAny(s, "\n\r\t\"'\\") {
		return parsedV6CIDR{}, fmt.Errorf("invalid characters")
	}
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		return parsedV6CIDR{}, err
	}
	if n.IP.To4() != nil {
		return parsedV6CIDR{}, fmt.Errorf("not ipv6")
	}
	ones, _ := n.Mask.Size()
	return parsedV6CIDR{ones: ones}, nil
}

func validateIPv6Addr(s string) error {
	s = strings.TrimSpace(s)
	ip := net.ParseIP(s)
	if ip == nil || ip.To4() != nil {
		return fmt.Errorf("invalid ipv6: %s", s)
	}
	return nil
}

// EffectiveDNS64Upstream 返回下发给客户端的 DNS64 解析器地址
func (n NatState) EffectiveDNS64Upstream() []string {
	if len(n.DNS64.Upstream) > 0 {
		return append([]string(nil), n.DNS64.Upstream...)
	}
	return []string{GoogleDNS64Primary, GoogleDNS64Secondary}
}

// DNS64UsesLocalUnbound 是否经本机 Unbound 提供 DNS64
func (n NatState) DNS64UsesLocalUnbound() bool {
	return n.Nat64Enabled && n.DNS64.Mode == DNS64ModeLocal
}

// DNS64UsesDnsmasqRelay 是否经 dnsmasq 转发到 Unbound 并向 DHCP 客户端下发 DNS
func (n NatState) DNS64UsesDnsmasqRelay() bool {
	return n.Nat64Enabled && n.DNS64.ServeToClients && n.DNS64UsesLocalUnbound()
}

// DNS64DirectToClients VPN/静态场景：客户端直连本机 Unbound 或公网 DNS64，不依赖 dnsmasq
func (n NatState) DNS64DirectToClients() bool {
	return n.Nat64Enabled && !n.DNS64.ServeToClients
}

// EffectiveUnboundListen 返回 Unbound 监听 host:port
func (d DNS64Config) EffectiveUnboundListen(gatewayIPv4 string) (host string, port int, err error) {
	if d.ServeToClients {
		s := strings.TrimSpace(d.UnboundListen)
		if s == "" {
			s = "127.0.0.1:5353"
		}
		return parseListenHostPort(s, 5353)
	}
	s := strings.TrimSpace(d.UnboundListen)
	if s != "" && !isLoopbackListen(s) {
		return parseListenHostPort(s, 53)
	}
	if gatewayIPv4 != "" {
		return gatewayIPv4, 53, nil
	}
	return "0.0.0.0", 53, nil
}

func isLoopbackListen(s string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	return strings.HasPrefix(s, "127.") || s == "::1" || strings.HasPrefix(s, "[::1]")
}

func parseListenHostPort(s string, defaultPort int) (host string, port int, err error) {
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", 0, fmt.Errorf("bad listen: %s", s)
		}
		host = s[1:end]
		rest := strings.TrimPrefix(s[end+1:], ":")
		if rest == "" {
			return host, defaultPort, nil
		}
		_, e := fmt.Sscanf(rest, "%d", &port)
		if e != nil {
			return "", 0, e
		}
		return host, port, nil
	}
	if strings.Count(s, ":") > 1 && !strings.Contains(s, ".") {
		return s, defaultPort, nil
	}
	host, ps, ok := strings.Cut(s, ":")
	if !ok {
		return s, defaultPort, nil
	}
	_, e := fmt.Sscanf(ps, "%d", &port)
	if e != nil {
		return "", 0, fmt.Errorf("bad port in %s", s)
	}
	return host, port, nil
}
