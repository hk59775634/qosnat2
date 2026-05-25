package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

// RateProfile 速率配置（API 字符串，如 8mbit）
type RateProfile struct {
	Down      string `json:"down"`
	Up        string `json:"up"`
	HostMask  int    `json:"host_mask,omitempty"`
}

// HostRate 单主机 /32 覆盖
type HostRate struct {
	Down string `json:"down"`
	Up   string `json:"up"`
}

// ShaperState 流量整形持久化（P1 起同步 BPF Map）
type ShaperState struct {
	Device          string                 `json:"device,omitempty"` // 默认绑定网卡，空则 DEV_LAN
	PolicyCIDR      string                 `json:"policy_cidr"`
	DefaultProfile  RateProfile            `json:"default_profile"`
	Profiles        []ProfileEntry         `json:"profiles"`
	Tenants         []TenantEntry          `json:"tenants,omitempty"`
	Hosts           map[string]HostRate    `json:"hosts,omitempty"` // 已废弃，启动时迁入 profiles
	Leaf            string                 `json:"leaf"`
	IdleTimeoutSec  int                    `json:"idle_timeout_sec"`
	FQFlows         int                    `json:"fq_flows,omitempty"`
	FQQuantum       int                    `json:"fq_quantum,omitempty"`
}

// ProfileEntry LPM 网段模板（ID 越小优先级越高，仅影响管理排序；数据面仍 LPM 最长前缀优先）
type ProfileEntry struct {
	CIDR     string `json:"cidr"`
	Down     string `json:"down"`
	Up       string `json:"up"`
	Mask     int    `json:"mask,omitempty"`
	ID       int    `json:"id"`
	Priority int    `json:"priority,omitempty"` // 已废弃，启动时迁入 id
	Device   string `json:"device,omitempty"`   // 绑定网卡，空则用 Shaper.Device 或 DEV_LAN
	TenantID string `json:"tenant_id,omitempty"` // P4 租户展开时标记，便于批量删除
}

// FirewallState 防火墙/NAT 扩展
type FirewallState struct {
	WanPortForwards []WanPortForward `json:"wan_port_forwards"`
	FilterRules     []FilterRule     `json:"filter_rules"`
	Aliases         []AliasSet `json:"aliases"`
}

// SystemState 系统可调项
type SystemState struct {
	Sysctl        map[string]string `json:"sysctl"`
	Hostname      string            `json:"hostname,omitempty"`
	TxQueueLenLAN int               `json:"txqueuelen_lan,omitempty"`
	TxQueueLenWAN int               `json:"txqueuelen_wan,omitempty"`
	RpsLAN             bool   `json:"rps_lan,omitempty"`
	RpsWAN             bool   `json:"rps_wan,omitempty"`
	PerfPreset         bool   `json:"perf_preset,omitempty"`
	TuningAutoApplied  bool   `json:"tuning_auto_applied,omitempty"`
	TuningTier         string `json:"tuning_tier,omitempty"`
	TLSEnabled         bool   `json:"tls_enabled,omitempty"`
}

// APIKey 持久化 API Key（仅存 key_hash；创建时明文仅返回一次）
type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	KeyHash   string `json:"key_hash,omitempty"`
	KeyPrefix string `json:"key_prefix,omitempty"`
	Key       string `json:"key,omitempty"` // 迁移用，保存前清空
	CreatedAt string `json:"created_at"`
}

// State 完整持久化（/var/lib/qosnat2/state.json）
type State struct {
	SetupComplete  bool              `json:"setup_complete"`
	AdminUser     string `json:"admin_user,omitempty"`
	AdminPassHash string `json:"admin_pass_hash,omitempty"`
	PolicyRoutes   []string          `json:"policy_routes"`
	Routes         []RouteEntry      `json:"routes"`
	SharedIPs      []string          `json:"shared_ips"`
	StaticMappings map[string]string `json:"static_mappings"`
	PrefixMappings map[string]string `json:"prefix_mappings"`
	Shaper         ShaperState       `json:"shaper"`
	Firewall       FirewallState     `json:"firewall"`
	System         SystemState       `json:"system"`
	DHCP           DHCPState         `json:"dhcp"`
	Network        NetworkState      `json:"network"`
	VPN            VPNState          `json:"vpn"`
	APIKeys        []APIKey          `json:"api_keys"`
}

// Store 线程安全状态
type Store struct {
	mu    sync.RWMutex
	path  string
	State State
}

// DefaultState 空状态默认值
func DefaultState() State {
	return State{
		SetupComplete:  false,
		PolicyRoutes:   []string{"10.0.0.0/8"},
		Routes:         []RouteEntry{},
		SharedIPs:      nil,
		StaticMappings: map[string]string{},
		PrefixMappings: map[string]string{},
		Shaper: ShaperState{
			PolicyCIDR: "10.0.0.0/8",
			DefaultProfile: RateProfile{
				Down: "8mbit", Up: "8mbit", HostMask: 32,
			},
			Profiles:       []ProfileEntry{},
			Hosts:          map[string]HostRate{},
			Leaf:           "fq_codel",
			IdleTimeoutSec: 300,
		},
		Firewall: FirewallState{
			WanPortForwards: []WanPortForward{},
			FilterRules:     []FilterRule{},
			Aliases: []AliasSet{},
		},
		System: SystemState{
			Sysctl: map[string]string{},
		},
		DHCP:    DefaultDHCP(),
		Network: NetworkState{Ifaces: []IfaceConfig{}, VLANs: []VLANIface{}, WanLinks: []WanLink{}},
		VPN: VPNState{
			WireGuard: WireGuardState{
				Enabled:    false,
				Interface:  "wg0",
				ListenPort: 51820,
				Address:    "10.200.0.1/24",
				Peers:      []WGPeer{},
			},
			OCServ: DefaultOCServ(),
		},
		APIKeys: []APIKey{},
	}
}

func New(path string) *Store {
	return &Store{path: path, State: DefaultState()}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			s.State = DefaultState()
			return nil
		}
		return err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}
	s.State = st
	s.ensureDefaultsLocked()
	return nil
}

func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	migrateAPIKeysLocked(&s.State.APIKeys)
	b, err := json.MarshalIndent(s.State, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0750); err != nil {
		return err
	}
	return os.WriteFile(s.path, b, 0600)
}

func (s *Store) Get() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.State
}

func (s *Store) Update(fn func(*State)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	fn(&s.State)
	s.ensureDefaultsLocked()
	return nil
}

func (s *Store) ensureDefaultsLocked() {
	if s.State.SharedIPs == nil {
		s.State.SharedIPs = []string{}
	}
	if s.State.StaticMappings == nil {
		s.State.StaticMappings = map[string]string{}
	}
	if s.State.PrefixMappings == nil {
		s.State.PrefixMappings = map[string]string{}
	}
	if s.State.Shaper.Hosts == nil {
		s.State.Shaper.Hosts = map[string]HostRate{}
	}
	if s.State.Shaper.Leaf == "" {
		s.State.Shaper.Leaf = "fq_codel"
	}
	if s.State.Shaper.IdleTimeoutSec == 0 {
		s.State.Shaper.IdleTimeoutSec = 300
	}
	if s.State.Routes == nil {
		s.State.Routes = []RouteEntry{}
	}
	if s.State.Shaper.PolicyCIDR == "" {
		s.State.Shaper.PolicyCIDR = "10.0.0.0/8"
	}
	if s.State.Shaper.DefaultProfile.Down == "" {
		s.State.Shaper.DefaultProfile = RateProfile{Down: "8mbit", Up: "8mbit", HostMask: 32}
	}
	def := DefaultDHCP()
	if s.State.DHCP.StaticLeases == nil {
		s.State.DHCP.StaticLeases = []DHCPStaticLease{}
	}
	if s.State.DHCP.DNSServers == nil {
		s.State.DHCP.DNSServers = def.DNSServers
	}
	if s.State.DHCP.LeaseTimeSec == 0 {
		s.State.DHCP.LeaseTimeSec = def.LeaseTimeSec
	}
	if s.State.DHCP.RangeStart == "" {
		s.State.DHCP.RangeStart = def.RangeStart
	}
	if s.State.DHCP.RangeEnd == "" {
		s.State.DHCP.RangeEnd = def.RangeEnd
	}
	if s.State.DHCP.Router == "" {
		s.State.DHCP.Router = def.Router
	}
	if s.State.DHCP.Netmask == "" {
		s.State.DHCP.Netmask = def.Netmask
	}
	if s.State.Firewall.WanPortForwards == nil {
		s.State.Firewall.WanPortForwards = []WanPortForward{}
	}
	if s.State.Firewall.FilterRules == nil {
		s.State.Firewall.FilterRules = []FilterRule{}
	}
	if s.State.Firewall.Aliases == nil {
		s.State.Firewall.Aliases = []AliasSet{}
	}
	if s.State.Network.Ifaces == nil {
		s.State.Network.Ifaces = []IfaceConfig{}
	}
	if s.State.Network.VLANs == nil {
		s.State.Network.VLANs = []VLANIface{}
	}
	if s.State.Network.WanLinks == nil {
		s.State.Network.WanLinks = []WanLink{}
	}
	MigrateWanForwards(&s.State.Firewall.WanPortForwards)
	if s.State.VPN.WireGuard.Interface == "" {
		s.State.VPN.WireGuard.Interface = "wg0"
	}
	if s.State.VPN.WireGuard.ListenPort == 0 {
		s.State.VPN.WireGuard.ListenPort = 51820
	}
	if s.State.VPN.WireGuard.Address == "" {
		s.State.VPN.WireGuard.Address = "10.200.0.1/24"
	}
	if s.State.VPN.WireGuard.Peers == nil {
		s.State.VPN.WireGuard.Peers = []WGPeer{}
	}
	if s.State.VPN.OCServ.TCPPort == 0 && len(s.State.VPN.OCServ.Users) == 0 && s.State.VPN.OCServ.IPv4Network == "" {
		s.State.VPN.OCServ = DefaultOCServ()
	} else {
		_ = NormalizeOCServ(&s.State.VPN.OCServ)
	}
	if s.State.VPN.OCServ.Users == nil {
		s.State.VPN.OCServ.Users = []OCServUser{}
	}
	MigrateHostsToProfiles(&s.State.Shaper.Profiles, s.State.Shaper.Hosts)
	s.State.Shaper.Hosts = nil
	NormalizeProfileIDs(&s.State.Shaper.Profiles)
	migrateAPIKeysLocked(&s.State.APIKeys)
}

// MbitToBPS 字节/秒（与 tc/htb 一致：mbit * 125000）
func MbitToBPS(rate string) (uint64, error) {
	rate = strings.TrimSpace(strings.ToLower(rate))
	if rate == "" {
		return 0, fmt.Errorf("empty rate")
	}
	mult := 125000.0
	suffix := "mbit"
	for _, suf := range []string{"gbit", "mbit", "kbit"} {
		if strings.HasSuffix(rate, suf) {
			suffix = suf
			rate = strings.TrimSpace(strings.TrimSuffix(rate, suf))
			break
		}
	}
	switch suffix {
	case "gbit":
		mult = 125000000
	case "kbit":
		mult = 125
	}
	n, err := strconv.ParseFloat(rate, 64)
	if err != nil {
		return 0, fmt.Errorf("parse rate %q: %w", rate, err)
	}
	return uint64(n * mult), nil
}
