package store

import (
	"fmt"
	"strings"
)

// OCServVhost 虚拟主机 [vhost:domain]（ocserv scope: vhost / vhost user）
type OCServVhost struct {
	Enabled  bool   `json:"enabled"`
	Domain   string `json:"domain"`
	Comment  string `json:"comment,omitempty"`
	AuthMethod string `json:"auth_method,omitempty"` // 空=继承；plain|radius|certificate

	// plain：独立密码文件（空则全局 ocpasswd）
	PlainPasswdPath string `json:"plain_passwd_path,omitempty"`
	// Users 仅用于 plain_passwd_path 非空时，写入该 vhost 密码文件
	Users []OCServUser `json:"users,omitempty"`

	// radius：Server 为空则继承全局 RADIUS；否则写入 /etc/radcli/vhosts/<domain>.conf
	Radius *OCServRadius `json:"radius,omitempty"`

	// TLS / 证书
	ManagedCertID  string `json:"managed_cert_id,omitempty"`
	ServerCertPath string `json:"server_cert_path,omitempty"`
	ServerKeyPath  string `json:"server_key_path,omitempty"`
	CaCertPath     string `json:"ca_cert_path,omitempty"`
	CRLPath        string `json:"crl_path,omitempty"`
	DHParamsPath   string `json:"dh_params_path,omitempty"`
	TLSPriorities  string `json:"tls_priorities,omitempty"`
	CertUserOID    string `json:"cert_user_oid,omitempty"`
	CertGroupOID   string `json:"cert_group_oid,omitempty"`

	// 网络 / 地址池
	IPv4Network   string   `json:"ipv4_network,omitempty"`
	IPv4Netmask   string   `json:"ipv4_netmask,omitempty"`
	IPv6Network   string   `json:"ipv6_network,omitempty"`
	IPv6Prefix    int      `json:"ipv6_prefix,omitempty"`
	DNS           []string `json:"dns,omitempty"`
	NBNS          []string `json:"nbns,omitempty"`
	DefaultDomain string   `json:"default_domain,omitempty"`
	TunnelAllDNS  bool     `json:"tunnel_all_dns,omitempty"`
	MTU           int      `json:"mtu,omitempty"`

	// 路由
	Routes        []string `json:"routes,omitempty"`
	NoRoutes      []string `json:"no_routes,omitempty"`
	IRoutes       []string `json:"iroutes,omitempty"`
	ExposeIRoutes bool     `json:"expose_iroutes,omitempty"`

	// 带宽
	RxDataPerSec int `json:"rx_data_per_sec,omitempty"`
	TxDataPerSec int `json:"tx_data_per_sec,omitempty"`
	PktMTUSize   int `json:"pkt_mtu_size,omitempty"`

	// 会话 / 超时
	IdleTimeout       int  `json:"idle_timeout,omitempty"`
	SessionTimeout    int  `json:"session_timeout,omitempty"`
	MobileIdleTimeout int  `json:"mobile_idle_timeout,omitempty"`
	MaxSameClients    int  `json:"max_same_clients,omitempty"`
	Keepalive         int  `json:"keepalive,omitempty"`
	DPD               int  `json:"dpd,omitempty"`
	MobileDPD         int  `json:"mobile_dpd,omitempty"`
	CookieTimeout     int  `json:"cookie_timeout,omitempty"`
	DenyRoaming       bool `json:"deny_roaming,omitempty"`
	PersistentCookies bool `json:"persistent_cookies,omitempty"`
	RekeyTime         int  `json:"rekey_time,omitempty"`
	RekeyMethod       string `json:"rekey_method,omitempty"`

	// 协议特性
	Compression       bool `json:"compression,omitempty"`
	PredictableIPs    bool `json:"predictable_ips,omitempty"`
	DtlsLegacy        bool `json:"dtls_legacy,omitempty"`
	CiscoClientCompat bool `json:"cisco_client_compat,omitempty"`
	CiscoSvcCompat    bool `json:"cisco_svc_client_compat,omitempty"`
	NoUDP             bool `json:"no_udp,omitempty"`

	// 伪装 / 横幅
	Banner           string `json:"banner,omitempty"`
	PreLoginBanner   string `json:"pre_login_banner,omitempty"`
	Camouflage       bool   `json:"camouflage,omitempty"`
	CamouflageSecret string `json:"camouflage_secret,omitempty"`
	CamouflageRealm  string `json:"camouflage_realm,omitempty"`

	// 按用户/组配置
	ConfigPerUser      string   `json:"config_per_user,omitempty"`
	ConfigPerGroup     string   `json:"config_per_group,omitempty"`
	DefaultUserConfig  string   `json:"default_user_config,omitempty"`
	DefaultGroupConfig string   `json:"default_group_config,omitempty"`
	SelectGroups       []string `json:"select_groups,omitempty"`
	AutoSelectGroup    bool     `json:"auto_select_group,omitempty"`
	DefaultSelectGroup string   `json:"default_select_group,omitempty"`

	// 脚本
	ConnectScript    string `json:"connect_script,omitempty"`
	DisconnectScript string `json:"disconnect_script,omitempty"`

	// 计费（空则继承全局 acct）
	AcctEnabled       bool `json:"acct_enabled,omitempty"`
	StatsReportTime   int  `json:"stats_report_time,omitempty"`
}

// VhostUsesOwnRadius 该 vhost 使用独立 RADIUS（非继承全局）；Radius != nil 即独立模式
func VhostUsesOwnRadius(v OCServVhost) bool {
	return v.Radius != nil
}

func normalizeVhostRadius(r *OCServRadius) error {
	if r == nil {
		return fmt.Errorf("radius config required")
	}
	if strings.TrimSpace(r.Server) == "" {
		return fmt.Errorf("radius server required")
	}
	if strings.TrimSpace(r.Secret) == "" {
		return fmt.Errorf("radius secret required")
	}
	if r.AuthPort <= 0 {
		r.AuthPort = 1812
	}
	if r.AcctPort <= 0 {
		r.AcctPort = 1813
	}
	if r.StatsReportTime <= 0 {
		r.StatsReportTime = 360
	}
	return nil
}
