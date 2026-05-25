package store

// OCServAdvanced ocserv 功能开关与高级参数（Apply 时写入 ocserv.conf）
type OCServAdvanced struct {
	TryMTUDiscovery   bool `json:"try_mtu_discovery"`
	IsolateWorkers    bool `json:"isolate_workers"`
	DtlsLegacy        bool `json:"dtls_legacy"`
	Tcp               bool `json:"tcp"`
	Udp               bool `json:"udp"`
	DenyRoaming       bool `json:"deny_roaming"`
	CiscoClientCompat bool `json:"cisco_client_compat"`
	CiscoSvcCompat    bool `json:"cisco_svc_client_compat"`
	ClientBypassProto bool `json:"client_bypass_protocol"`
	Compression       bool `json:"compression"`
	Keepalive         bool `json:"keepalive"`
	DPD               bool `json:"dpd"`
	MobileDPD         bool `json:"mobile_dpd"`
	PredictableIPs    bool `json:"predictable_ips"`
	PingLeases        bool `json:"ping_leases"`
	UseOcctl          bool `json:"use_occtl"`
	Rekey             bool `json:"rekey"`
	SwitchToTcp       bool `json:"switch_to_tcp"`
	Camouflage        bool `json:"camouflage"`

	MaxSameClients     int    `json:"max_same_clients,omitempty"`
	KeepaliveSec       int    `json:"keepalive_sec,omitempty"`
	DPDSec             int    `json:"dpd_sec,omitempty"`
	MobileDPDSec       int    `json:"mobile_dpd_sec,omitempty"`
	CookieTimeout      int    `json:"cookie_timeout,omitempty"`
	RekeyTime          int    `json:"rekey_time,omitempty"`
	RekeyMethod        string `json:"rekey_method,omitempty"` // ssl | new-tunnel
	AuthTimeout        int    `json:"auth_timeout,omitempty"`
	SwitchToTcpTimeout int    `json:"switch_to_tcp_timeout,omitempty"`

	RateLimitMs          int `json:"rate_limit_ms,omitempty"`
	LogLevel             int `json:"log_level,omitempty"`
	MaxBanScore          int `json:"max_ban_score,omitempty"`
	BanTime              int `json:"ban_time,omitempty"`
	BanResetTime         int `json:"ban_reset_time,omitempty"`
	ServerStatsResetTime int `json:"server_stats_reset_time,omitempty"`

	RxDataPerSec int `json:"rx_data_per_sec,omitempty"` // 0 = 不写入
	TxDataPerSec int `json:"tx_data_per_sec,omitempty"`

	CamouflageSecret string `json:"camouflage_secret,omitempty"` // GET 不返回
	CamouflageRealm  string `json:"camouflage_realm,omitempty"`

	DefaultDomain    string `json:"default_domain,omitempty"`
	ConfigPerGroup   string `json:"config_per_group,omitempty"`
	CertUserOID      string `json:"cert_user_oid,omitempty"`
	TLSPriorities    string `json:"tls_priorities,omitempty"`
}

func DefaultOCServAdvanced() OCServAdvanced {
	return OCServAdvanced{
		TryMTUDiscovery:      true,
		IsolateWorkers:       true,
		DtlsLegacy:           true,
		Tcp:                  true,
		Udp:                  true,
		DenyRoaming:          false,
		CiscoClientCompat:    true,
		CiscoSvcCompat:       false,
		ClientBypassProto:    false,
		Compression:          false,
		Keepalive:            true,
		DPD:                  true,
		MobileDPD:            true,
		PredictableIPs:       false,
		PingLeases:           false,
		UseOcctl:             false,
		Rekey:                true,
		SwitchToTcp:          true,
		Camouflage:           false,
		MaxSameClients:       2,
		KeepaliveSec:         32400,
		DPDSec:               90,
		MobileDPDSec:         1800,
		CookieTimeout:        300,
		RekeyTime:            172800,
		RekeyMethod:          "ssl",
		AuthTimeout:          240,
		SwitchToTcpTimeout:   25,
		RateLimitMs:          100,
		LogLevel:             2,
		MaxBanScore:          80,
		BanTime:              300,
		BanResetTime:         1200,
		ServerStatsResetTime: 604800,
		CertUserOID:          "0.9.2342.19200300.100.1.1",
		TLSPriorities:        "NORMAL:%SERVER_PRECEDENCE:%COMPAT:-VERS-SSL3.0:-VERS-TLS1.0:-VERS-TLS1.1",
	}
}

func ocservAdvancedEmpty(a OCServAdvanced) bool {
	return a == OCServAdvanced{}
}

// MergeOCServAdvanced 补全高级配置默认值；legacyIsolate 为旧版顶层 isolate_workers
func MergeOCServAdvanced(dst *OCServAdvanced, legacyIsolate *bool) {
	def := DefaultOCServAdvanced()
	if ocservAdvancedEmpty(*dst) {
		*dst = def
		if legacyIsolate != nil && *legacyIsolate {
			dst.IsolateWorkers = true
		}
		return
	}
	mergeIntDefault(&dst.MaxSameClients, def.MaxSameClients)
	mergeIntDefault(&dst.KeepaliveSec, def.KeepaliveSec)
	mergeIntDefault(&dst.DPDSec, def.DPDSec)
	mergeIntDefault(&dst.MobileDPDSec, def.MobileDPDSec)
	mergeIntDefault(&dst.CookieTimeout, def.CookieTimeout)
	mergeIntDefault(&dst.RekeyTime, def.RekeyTime)
	mergeIntDefault(&dst.AuthTimeout, def.AuthTimeout)
	mergeIntDefault(&dst.SwitchToTcpTimeout, def.SwitchToTcpTimeout)
	mergeIntDefault(&dst.RateLimitMs, def.RateLimitMs)
	mergeIntDefault(&dst.LogLevel, def.LogLevel)
	mergeIntDefault(&dst.MaxBanScore, def.MaxBanScore)
	mergeIntDefault(&dst.BanTime, def.BanTime)
	mergeIntDefault(&dst.BanResetTime, def.BanResetTime)
	mergeIntDefault(&dst.ServerStatsResetTime, def.ServerStatsResetTime)
	if dst.RekeyMethod == "" {
		dst.RekeyMethod = def.RekeyMethod
	}
	if dst.CertUserOID == "" {
		dst.CertUserOID = def.CertUserOID
	}
	if dst.TLSPriorities == "" {
		dst.TLSPriorities = def.TLSPriorities
	}
}

func mergeIntDefault(dst *int, def int) {
	if *dst <= 0 {
		*dst = def
	}
}
