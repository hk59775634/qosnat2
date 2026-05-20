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
	PolicyCIDR      string                 `json:"policy_cidr"`
	DefaultProfile  RateProfile            `json:"default_profile"`
	Profiles        []ProfileEntry         `json:"profiles"`
	Hosts           map[string]HostRate    `json:"hosts"`
	Leaf            string                 `json:"leaf"`
	IdleTimeoutSec  int                    `json:"idle_timeout_sec"`
}

// ProfileEntry LPM 网段模板
type ProfileEntry struct {
	CIDR string      `json:"cidr"`
	Down string      `json:"down"`
	Up   string      `json:"up"`
	Mask int         `json:"mask,omitempty"`
}

// WanPortForward 公网端口转发
type WanPortForward struct {
	Proto    string `json:"proto"`
	WanPort  int    `json:"wan_port"`
	HostIP   string `json:"host_ip"`
	HostPort int    `json:"host_port"`
	Comment  string `json:"comment,omitempty"`
}

// FirewallState 防火墙/NAT 扩展
type FirewallState struct {
	WanPortForwards []WanPortForward `json:"wan_port_forwards"`
	Rules           []any          `json:"rules,omitempty"`
}

// SystemState 系统可调项
type SystemState struct {
	Sysctl   map[string]string `json:"sysctl"`
	Hostname string            `json:"hostname,omitempty"`
}

// APIKey 持久化 API Key
type APIKey struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Key       string `json:"key"`
	CreatedAt string `json:"created_at"`
}

// State 完整持久化（/var/lib/qosnat2/state.json）
type State struct {
	PolicyRoutes   []string          `json:"policy_routes"`
	SharedIPs      []string          `json:"shared_ips"`
	StaticMappings map[string]string `json:"static_mappings"`
	PrefixMappings map[string]string `json:"prefix_mappings"`
	Shaper         ShaperState       `json:"shaper"`
	Firewall       FirewallState     `json:"firewall"`
	System         SystemState       `json:"system"`
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
		PolicyRoutes:   []string{"10.0.0.0/8"},
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
			Rules:           []any{},
		},
		System: SystemState{
			Sysctl: map[string]string{},
		},
		VPN: VPNState{
			WireGuard: WireGuardState{
				Enabled:    false,
				Interface:  "wg0",
				ListenPort: 51820,
				Address:    "10.200.0.1/24",
				Peers:      []WGPeer{},
			},
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
	s.mu.RLock()
	b, err := json.MarshalIndent(s.State, "", "  ")
	s.mu.RUnlock()
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

func (s *Store) ensureDefaults() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ensureDefaultsLocked()
}

func (s *Store) ensureDefaultsLocked() {
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
	if s.State.Shaper.PolicyCIDR == "" {
		s.State.Shaper.PolicyCIDR = "10.0.0.0/8"
	}
	if s.State.Shaper.DefaultProfile.Down == "" {
		s.State.Shaper.DefaultProfile = RateProfile{Down: "8mbit", Up: "8mbit", HostMask: 32}
	}
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
