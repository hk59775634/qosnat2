package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// OCServAuthMethod 认证方式：plain（本地 ocpasswd）或 radius
const (
	OCServAuthPlain  = "plain"
	OCServAuthRadius = "radius"
)

// OCServRadius RADIUS 参数（radcli；Apply 时写入 /etc/radcli）
type OCServRadius struct {
	Server          string `json:"server"`                      // RADIUS 主机名或 IP
	AuthPort        int    `json:"auth_port,omitempty"`         // 默认 1812
	AcctPort        int    `json:"acct_port,omitempty"`         // 默认 1813
	Secret          string `json:"secret,omitempty"`            // 共享密钥；GET 不返回
	GroupConfig     bool   `json:"groupconfig,omitempty"`       // 从 RADIUS 读取 per-user 配置
	NASIdentifier   string `json:"nas_identifier,omitempty"`    // NAS-Identifier 属性
	AcctEnabled     bool   `json:"acct_enabled,omitempty"`      // RADIUS 计费
	StatsReportTime int    `json:"stats_report_time,omitempty"` // 计费上报间隔（秒），默认 360
	ConfigPath      string `json:"config_path,omitempty"`       // 覆盖 radcli 配置路径
}

// OCServUser OpenConnect 用户（plain 认证，Apply 时写入 ocpasswd）
type OCServUser struct {
	Username     string `json:"username"`
	Password     string `json:"password,omitempty"` // 仅创建/改密时提交；列表 GET 不返回
	Group        string `json:"group,omitempty"`
	Comment      string `json:"comment,omitempty"`
	TotalRxBytes uint64 `json:"total_rx_bytes,omitempty"` // GET 列表：历史累计下行（采样）
	TotalTxBytes uint64 `json:"total_tx_bytes,omitempty"` // GET 列表：历史累计上行
	TotalBytes   uint64 `json:"total_bytes,omitempty"`    // GET 列表：上下行合计
}

// OCServState ocserv 服务端配置（持久化在 state.json）
type OCServState struct {
	Enabled        bool         `json:"enabled"`
	AuthMethod     string       `json:"auth_method,omitempty"` // plain | radius
	Radius         OCServRadius `json:"radius,omitempty"`
	TCPPort        int          `json:"tcp_port"`
	UDPPort        int          `json:"udp_port"`
	Device         string       `json:"device"` // tun 设备名前缀，默认 vpns
	IPv4Network    string       `json:"ipv4_network"`
	IPv4Netmask    string       `json:"ipv4_netmask"`
	DNS            []string       `json:"dns,omitempty"`
	Routes         []string       `json:"routes,omitempty"`    // 如 default、10.0.0.0/24
	NoRoutes       []string       `json:"no_routes,omitempty"` // no-route
	MaxClients     int            `json:"max_clients,omitempty"`
	IsolateWorkers bool           `json:"isolate_workers,omitempty"` // 已迁移至 advanced，仅作旧 state 兼容
	Advanced       OCServAdvanced `json:"advanced,omitempty"`
	UseQoSnatTLS    bool   `json:"use_qosnat_tls,omitempty"`
	ManagedCertID   string `json:"managed_cert_id,omitempty"`
	ServerCertPath string         `json:"server_cert_path,omitempty"`
	ServerKeyPath  string         `json:"server_key_path,omitempty"`
	CaCertPath     string         `json:"ca_cert_path,omitempty"`
	SocketFile     string         `json:"socket_file,omitempty"`
	ServerCert     string         `json:"server_cert,omitempty"` // PEM 内联（少用）
	ServerKey      string         `json:"server_key,omitempty"`
	Users              []OCServUser   `json:"users"`
	ConfigPerGroup     string         `json:"config_per_group,omitempty"`
	ConfigPerUser      string         `json:"config_per_user,omitempty"`
	DefaultGroupConfig string         `json:"default_group_config,omitempty"`
	DefaultUserConfig  string         `json:"default_user_config,omitempty"`
	AutoSelectGroup    bool           `json:"auto_select_group,omitempty"`
	DefaultSelectGroup string         `json:"default_select_group,omitempty"`
	Groups             []OCServGroup  `json:"groups,omitempty"`
	Vhosts             []OCServVhost  `json:"vhosts,omitempty"`
}

func DefaultOCServ() OCServState {
	return OCServState{
		Enabled:        false,
		AuthMethod:     OCServAuthPlain,
		Radius: OCServRadius{
			AuthPort:        1812,
			AcctPort:        1813,
			GroupConfig:     true,
			StatsReportTime: 360,
		},
		TCPPort:        443,
		UDPPort:        443,
		Device:         "vpns",
		IPv4Network:    "10.250.0.0",
		IPv4Netmask:    "255.255.255.0",
		DNS:            []string{"8.8.8.8"},
		Routes:         []string{"default"},
		MaxClients:     128,
		Advanced:       DefaultOCServAdvanced(),
		UseQoSnatTLS:   true,
		ServerCertPath: "/etc/ocserv/certs/server-cert.pem",
		ServerKeyPath:  "/etc/ocserv/certs/server-key.pem",
		SocketFile:     "/var/run/ocserv-socket",
		Users:              []OCServUser{},
		ConfigPerGroup:     "/etc/ocserv/config-per-group/",
		DefaultGroupConfig: "/etc/ocserv/defaults/group.conf",
		Groups:             []OCServGroup{},
		Vhosts:             []OCServVhost{},
	}
}

// OCServUsesRadius 是否使用 RADIUS 认证
func OCServUsesRadius(o OCServState) bool {
	return strings.TrimSpace(o.AuthMethod) == OCServAuthRadius
}

// NormalizeOCServ 校验 ocserv 配置
func NormalizeOCServ(o *OCServState) error {
	if o == nil {
		return fmt.Errorf("ocserv nil")
	}
	am := strings.TrimSpace(o.AuthMethod)
	if am == "" {
		o.AuthMethod = OCServAuthPlain
	} else if am != OCServAuthPlain && am != OCServAuthRadius {
		return fmt.Errorf("auth_method must be plain or radius")
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
	if o.DNS == nil {
		o.DNS = []string{"8.8.8.8"}
	}
	o.DNS = trimStringList(o.DNS)
	if o.Routes == nil {
		o.Routes = []string{"default"}
	}
	o.Routes = trimStringList(o.Routes)
	o.NoRoutes = trimStringList(o.NoRoutes)
	if strings.TrimSpace(o.ServerCertPath) == "" {
		o.ServerCertPath = "/etc/ocserv/certs/server-cert.pem"
	}
	if strings.TrimSpace(o.ServerKeyPath) == "" {
		o.ServerKeyPath = "/etc/ocserv/certs/server-key.pem"
	}
	if strings.TrimSpace(o.SocketFile) == "" {
		o.SocketFile = "/var/run/ocserv-socket"
	}
	if o.MaxClients <= 0 {
		o.MaxClients = 128
	}
	var legacyIso *bool
	if ocservAdvancedEmpty(o.Advanced) {
		legacyIso = &o.IsolateWorkers
	}
	MergeOCServAdvanced(&o.Advanced, legacyIso)
	o.IsolateWorkers = false
	if !o.Advanced.Tcp && !o.Advanced.Udp {
		return fmt.Errorf("TCP 与 UDP 至少启用一项")
	}
	rm := strings.TrimSpace(o.Advanced.RekeyMethod)
	if rm != "" && rm != "ssl" && rm != "new-tunnel" {
		return fmt.Errorf("rekey_method must be ssl or new-tunnel")
	}
	if o.Advanced.Camouflage && strings.TrimSpace(o.Advanced.CamouflageSecret) == "" {
		return fmt.Errorf("camouflage enabled requires camouflage_secret")
	}
	if o.Radius.AuthPort <= 0 {
		o.Radius.AuthPort = 1812
	}
	if o.Radius.AcctPort <= 0 {
		o.Radius.AcctPort = 1813
	}
	if o.Radius.StatsReportTime <= 0 {
		o.Radius.StatsReportTime = 360
	}
	if OCServUsesRadius(*o) {
		if strings.TrimSpace(o.Radius.Server) == "" {
			return fmt.Errorf("radius server required")
		}
		if strings.TrimSpace(o.Radius.Secret) == "" {
			return fmt.Errorf("radius secret required")
		}
		o.Users = nil
	} else {
		o.Radius.Secret = ""
	}
	for i := range o.Users {
		u := strings.TrimSpace(o.Users[i].Username)
		if u == "" {
			return fmt.Errorf("user username required")
		}
		o.Users[i].Username = u
		o.Users[i].Group = strings.TrimSpace(o.Users[i].Group)
		o.Users[i].TotalRxBytes = 0
		o.Users[i].TotalTxBytes = 0
		o.Users[i].TotalBytes = 0
	}
	if strings.TrimSpace(o.ConfigPerGroup) == "" && strings.TrimSpace(o.Advanced.ConfigPerGroup) != "" {
		o.ConfigPerGroup = strings.TrimSpace(o.Advanced.ConfigPerGroup)
	}
	if strings.TrimSpace(o.ConfigPerGroup) == "" {
		o.ConfigPerGroup = "/etc/ocserv/config-per-group/"
	}
	if strings.TrimSpace(o.DefaultGroupConfig) == "" {
		o.DefaultGroupConfig = "/etc/ocserv/defaults/group.conf"
	}
	if o.Groups == nil {
		o.Groups = []OCServGroup{}
	}
	if o.Vhosts == nil {
		o.Vhosts = []OCServVhost{}
	}
	if err := NormalizeOCServGroups(&o.Groups); err != nil {
		return err
	}
	if err := NormalizeOCServVhosts(&o.Vhosts, o.AuthMethod); err != nil {
		return err
	}
	for _, u := range o.Users {
		if u.Group == "" {
			continue
		}
		found := false
		for _, g := range o.Groups {
			if g.Name == u.Group {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("user %s: unknown group %s", u.Username, u.Group)
		}
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

func trimStringList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func NewOCServUserID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return "oc-" + hex.EncodeToString(b[:])
}
