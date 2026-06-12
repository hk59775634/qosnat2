package store

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
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
	Enabled         bool                   `json:"enabled"` // false = 纯 NAT，不加载 TC/eBPF 整形
	Mode            string                 `json:"mode,omitempty"` // 省略即 EDT；旧 htb 在加载时自动清除
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
	WanPortForwards    []WanPortForward `json:"wan_port_forwards"`
	FilterRules        []FilterRule     `json:"filter_rules"`
	PendingFilterDraft bool             `json:"pending_filter_draft,omitempty"`
	PendingFilterRules []FilterRule     `json:"pending_filter_rules,omitempty"`
	Aliases            []AliasSet       `json:"aliases"`
	// MaxSessionsPerIP 每内网源 IP 最大出站 conntrack 会话数；0=不限制。
	MaxSessionsPerIP int `json:"max_sessions_per_ip,omitempty"`
}

// DefaultDisplayName 控制台 UI 默认产品名（非功能标识）
const DefaultDisplayName = "qosnat2"

// EffectiveDisplayName 返回用于 UI 展示的系统名称
func EffectiveDisplayName(name string) string {
	n := strings.TrimSpace(name)
	if n == "" {
		return DefaultDisplayName
	}
	if len(n) > 64 {
		return n[:64]
	}
	return n
}

// SystemState 系统可调项
type SystemState struct {
	Sysctl        map[string]string `json:"sysctl"`
	Hostname      string            `json:"hostname,omitempty"`
	DisplayName   string            `json:"display_name,omitempty"` // UI 品牌名，不影响服务标识
	TxQueueLenLAN int               `json:"txqueuelen_lan,omitempty"`
	TxQueueLenWAN int               `json:"txqueuelen_wan,omitempty"`
	RpsLAN             bool   `json:"rps_lan,omitempty"`
	RpsWAN             bool   `json:"rps_wan,omitempty"`
	PerfPreset         bool   `json:"perf_preset,omitempty"`
	TuningAutoApplied  bool   `json:"tuning_auto_applied,omitempty"`
	TuningTier         string `json:"tuning_tier,omitempty"`
	TLSEnabled            bool   `json:"tls_enabled,omitempty"`
	TLSDomain             string `json:"tls_domain,omitempty"`
	TLSAcmeEnabled        bool   `json:"tls_acme_enabled,omitempty"`
	TLSAcmeEmail          string `json:"tls_acme_email,omitempty"`
	TLSAcmeStaging        bool   `json:"tls_acme_staging,omitempty"`
	TLSAcmeRenewDays      int    `json:"tls_acme_renew_days,omitempty"` // 到期前 N 天续期，默认 30
	TLSAcmeLastOK         string `json:"tls_acme_last_ok,omitempty"`
	TLSAcmeLastError      string `json:"tls_acme_last_error,omitempty"`
	TLSManagedCertID      string `json:"tls_managed_cert_id,omitempty"`
	// AcmeTempAllowHTTP01 在执行 HTTP-01 验证期间临时放开 tcp/80 入站访问。
	// 该值由服务端在完成 ACME obtain/renew 后会自动恢复，不建议手动修改。
	AcmeTempAllowHTTP01 bool `json:"acme_temp_allow_http01,omitempty"`
	// AcmeTempAllowHTTP01IPs HTTP-01 期间放行的本机目标 IPv4（域名 DNS 解析与本机地址交集）。
	AcmeTempAllowHTTP01IPs []string `json:"acme_temp_allow_http01_ips,omitempty"`
	// DiagnosticsTerminalEnabled 为 true 时允许 Web Terminal（默认 false，等同 root shell）。
	DiagnosticsTerminalEnabled bool `json:"diagnostics_terminal_enabled,omitempty"`
	// RouteBackend 托管路由下发方式：kernel（ip -batch）| frr（staticd/zebra）。
	RouteBackend string `json:"route_backend,omitempty"`
	// FrrBootOnStartup 保存时是否 systemctl enable frr。
	FrrBootOnStartup bool `json:"frr_boot_on_startup,omitempty"`
}

// APIKey 持久化 API Key（仅存 key_hash；创建时明文仅返回一次）
type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role,omitempty"` // admin（默认）| readonly | firewall
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
	Nat            NatState          `json:"nat"`
	Routes          []RouteEntry          `json:"routes"`
	DynamicRouting  DynamicRoutingState   `json:"dynamic_routing,omitempty"`
	Shaper          ShaperState           `json:"shaper"`
	Firewall       FirewallState     `json:"firewall"`
	System         SystemState       `json:"system"`
	DHCP           DHCPState         `json:"dhcp"`
	SNMP           SNMPState         `json:"snmp,omitempty"`
	Network        NetworkState      `json:"network"`
	VPN            VPNState          `json:"vpn"`
	APIKeys        []APIKey             `json:"api_keys"`
	Certificates   []ManagedCertificate `json:"certificates,omitempty"`
	Notifications  []UINotification     `json:"notifications,omitempty"`
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
		Nat:    DefaultNat(),
		Routes: []RouteEntry{},
		Shaper: ShaperState{
			PolicyCIDR: "10.0.0.0/8",
			DefaultProfile: RateProfile{
				HostMask: 32,
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
		SNMP:    DefaultSNMP(),
		Network: NetworkState{Ifaces: []IfaceConfig{}, VLANs: []VLANIface{}, WanLinks: []WanLink{}, EgressPolicies: []EgressPolicy{}},
		VPN: VPNState{
			WireGuards: []WireGuardInstance{
				{
					ID:   "default",
					Name: "default",
					Mode: WGModeServer,
					WireGuardState: WireGuardState{
						Enabled:    false,
						Interface:  "wg0",
						ListenPort: 51820,
						Address:    "10.200.0.1/24",
						Peers:      []WGPeer{},
					},
				},
			},
			OCServ: DefaultOCServ(),
		},
		APIKeys:      []APIKey{},
		Certificates:  []ManagedCertificate{},
		Notifications: []UINotification{},
	}
}

func New(path string) *Store {
	return &Store{path: path, State: DefaultState()}
}

func (s *Store) Path() string { return s.path }

func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

func (s *Store) loadLocked() error {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return s.loadFromBackupOrDefault(fmt.Errorf("state file missing"))
		}
		return s.loadFromBackupOrError(err)
	}
	if err := s.applyStateJSON(b); err != nil {
		return s.loadFromBackupOrError(err)
	}
	return nil
}

func (s *Store) applyStateJSON(b []byte) error {
	var disk struct {
		State
		Legacy natLegacyFields `json:"-"`
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}
	if err := json.Unmarshal(b, &disk.State); err != nil {
		return err
	}
	for _, key := range []string{"policy_routes", "shared_ips", "static_mappings", "prefix_mappings"} {
		if v, ok := raw[key]; ok {
			var unmarshalErr error
			switch key {
			case "policy_routes":
				unmarshalErr = json.Unmarshal(v, &disk.Legacy.PolicyRoutes)
			case "shared_ips":
				unmarshalErr = json.Unmarshal(v, &disk.Legacy.SharedIPs)
			case "static_mappings":
				unmarshalErr = json.Unmarshal(v, &disk.Legacy.StaticMappings)
			case "prefix_mappings":
				unmarshalErr = json.Unmarshal(v, &disk.Legacy.PrefixMappings)
			}
			if unmarshalErr != nil {
				log.Printf("state.json legacy field %q: %v", key, unmarshalErr)
			}
		}
	}
	prevShaperMode := strings.TrimSpace(disk.State.Shaper.Mode)
	s.State = disk.State
	MigrateNatFromLegacy(&s.State, disk.Legacy)
	if rawShaper, ok := raw["shaper"]; ok {
		MigrateShaperEnabled(rawShaper, &s.State.Shaper)
	}
	s.ensureDefaultsLocked()
	if prevShaperMode != "" && strings.TrimSpace(s.State.Shaper.Mode) == "" {
		out, err := json.MarshalIndent(s.State, "", "  ")
		if err != nil {
			log.Printf("state: marshal shaper mode migration: %v", err)
		} else if err := WriteFileAtomic(s.path, out, 0600); err != nil {
			log.Printf("state: persist shaper mode migration: %v", err)
		} else {
			_ = os.WriteFile(s.path+".bak", out, 0600)
			log.Printf("state: cleared legacy shaper.mode %q (EDT default)", prevShaperMode)
		}
	}
	return nil
}

func (s *Store) backupPath() string { return s.path + ".bak" }

func (s *Store) loadFromBackupOrError(loadErr error) error {
	b, err := os.ReadFile(s.backupPath())
	if err != nil {
		return loadErr
	}
	if err := s.applyStateJSON(b); err != nil {
		return loadErr
	}
	log.Printf("state: recovered from %s (%v)", s.backupPath(), loadErr)
	if err := WriteFileAtomic(s.path, b, 0600); err != nil {
		log.Printf("state: restore main file from backup failed: %v", err)
	}
	return nil
}

func (s *Store) loadFromBackupOrDefault(loadErr error) error {
	b, err := os.ReadFile(s.backupPath())
	if err != nil {
		s.State = DefaultState()
		return nil
	}
	if err := s.applyStateJSON(b); err != nil {
		s.State = DefaultState()
		return nil
	}
	log.Printf("state: no main file; loaded from %s", s.backupPath())
	if err := WriteFileAtomic(s.path, b, 0600); err != nil {
		log.Printf("state: create main file from backup failed: %v", err)
	}
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
	path := s.path
	if err := WriteFileAtomic(path, b, 0600); err != nil {
		return err
	}
	// 保留一份 .bak 便于崩溃后人工恢复
	_ = os.WriteFile(path+".bak", b, 0600)
	return nil
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
	ensureNatDefaults(&s.State.Nat)
	if s.State.Shaper.Hosts == nil {
		s.State.Shaper.Hosts = map[string]HostRate{}
	}
	MigrateLegacyShaperMode(&s.State.Shaper)
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
	if s.State.Shaper.DefaultProfile.HostMask == 0 && RateProfileUnlimited(s.State.Shaper.DefaultProfile) {
		s.State.Shaper.DefaultProfile.HostMask = 32
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
	if s.State.DHCP.ChnroutesFile == "" {
		s.State.DHCP.ChnroutesFile = def.ChnroutesFile
	}
	if s.State.DHCP.TrustedDNS == nil {
		s.State.DHCP.TrustedDNS = []string{}
	}
	if s.State.DHCP.UntrustedDNS == nil {
		s.State.DHCP.UntrustedDNS = []string{}
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
	if s.State.Network.EgressPolicies == nil {
		s.State.Network.EgressPolicies = []EgressPolicy{}
	}
	MigrateWanForwards(&s.State.Firewall.WanPortForwards)
	MigrateLegacyWireGuardToInstances(&s.State.VPN)
	if s.State.VPN.WireGuards == nil {
		s.State.VPN.WireGuards = []WireGuardInstance{}
	}
	if len(s.State.VPN.WireGuards) == 0 {
		inst := WireGuardInstance{ID: "default", Name: "default", Mode: WGModeServer}
		NormalizeWireGuardInstance(&inst)
		s.State.VPN.WireGuards = []WireGuardInstance{inst}
	}
	for i := range s.State.VPN.WireGuards {
		NormalizeWireGuardInstance(&s.State.VPN.WireGuards[i])
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
