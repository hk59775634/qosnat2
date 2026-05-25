package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// OCServUser OpenConnect 用户（plain 认证，Apply 时写入 ocpasswd）
type OCServUser struct {
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // 仅创建/改密时提交；列表 GET 不返回
	Group    string `json:"group,omitempty"`
	Comment  string `json:"comment,omitempty"`
}

// OCServState ocserv 服务端配置（持久化在 state.json）
type OCServState struct {
	Enabled        bool         `json:"enabled"`
	TCPPort        int          `json:"tcp_port"`
	UDPPort        int          `json:"udp_port"`
	Device         string       `json:"device"` // tun 设备名前缀，默认 vpns
	IPv4Network    string       `json:"ipv4_network"`
	IPv4Netmask    string       `json:"ipv4_netmask"`
	DNS            []string     `json:"dns,omitempty"`
	Routes         []string     `json:"routes,omitempty"` // 如 default、192.168.0.0/24
	MaxClients     int          `json:"max_clients,omitempty"`
	IsolateWorkers bool         `json:"isolate_workers,omitempty"`
	UseQoSnatTLS   bool         `json:"use_qosnat_tls,omitempty"` // 证书用 /etc/qosnat2/tls.*
	ServerCert     string       `json:"server_cert,omitempty"`
	ServerKey      string       `json:"server_key,omitempty"`
	Users          []OCServUser `json:"users"`
}

func DefaultOCServ() OCServState {
	return OCServState{
		Enabled:        false,
		TCPPort:        443,
		UDPPort:        443,
		Device:         "vpns",
		IPv4Network:    "10.250.0.0",
		IPv4Netmask:    "255.255.255.0",
		DNS:            []string{"8.8.8.8"},
		Routes:         []string{"default"},
		MaxClients:     128,
		IsolateWorkers: true,
		UseQoSnatTLS:   true,
		Users:          []OCServUser{},
	}
}

// NormalizeOCServ 校验 ocserv 配置
func NormalizeOCServ(o *OCServState) error {
	if o == nil {
		return fmt.Errorf("ocserv nil")
	}
	if o.TCPPort <= 0 {
		o.TCPPort = 443
	}
	if o.UDPPort <= 0 {
		o.UDPPort = o.TCPPort
	}
	if strings.TrimSpace(o.Device) == "" {
		o.Device = "vpns"
	}
	if strings.TrimSpace(o.IPv4Network) == "" {
		o.IPv4Network = "10.250.0.0"
	}
	if strings.TrimSpace(o.IPv4Netmask) == "" {
		o.IPv4Netmask = "255.255.255.0"
	}
	if err := validateIPv4Pool(o.IPv4Network, o.IPv4Netmask); err != nil {
		return err
	}
	if len(o.DNS) == 0 {
		o.DNS = []string{"8.8.8.8"}
	}
	if len(o.Routes) == 0 {
		o.Routes = []string{"default"}
	}
	if o.MaxClients <= 0 {
		o.MaxClients = 128
	}
	for i := range o.Users {
		u := strings.TrimSpace(o.Users[i].Username)
		if u == "" {
			return fmt.Errorf("user username required")
		}
		o.Users[i].Username = u
	}
	return nil
}

func validateIPv4Pool(network, netmask string) error {
	ip := net.ParseIP(strings.TrimSpace(network))
	if ip == nil || ip.To4() == nil {
		return fmt.Errorf("invalid ipv4_network")
	}
	m := net.IPMask(net.ParseIP(strings.TrimSpace(netmask)).To4())
	if m == nil {
		return fmt.Errorf("invalid ipv4_netmask")
	}
	ones, bits := m.Size()
	if ones == 0 || bits != 32 {
		return fmt.Errorf("invalid ipv4_netmask")
	}
	return nil
}

func NewOCServUserID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "oc-" + hex.EncodeToString(b[:])
}
