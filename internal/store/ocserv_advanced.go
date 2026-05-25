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
	Compression       bool `json:"compression"`
	Keepalive         bool `json:"keepalive"`
	DPD               bool `json:"dpd"`
	MobileDPD         bool `json:"mobile_dpd"`
	PredictableIPs    bool `json:"predictable_ips"`
	PingLeases        bool `json:"ping_leases"`
	UseOcctl          bool `json:"use_occtl"`
	Rekey             bool `json:"rekey"`
	SwitchToTcp       bool `json:"switch_to_tcp"`

	MaxSameClients     int `json:"max_same_clients,omitempty"`
	KeepaliveSec       int `json:"keepalive_sec,omitempty"`
	DPDSec             int `json:"dpd_sec,omitempty"`
	MobileDPDSec       int `json:"mobile_dpd_sec,omitempty"`
	CookieTimeout      int `json:"cookie_timeout,omitempty"`
	RekeyTime          int `json:"rekey_time,omitempty"`
	AuthTimeout        int `json:"auth_timeout,omitempty"`
	SwitchToTcpTimeout int `json:"switch_to_tcp_timeout,omitempty"`
}

func DefaultOCServAdvanced() OCServAdvanced {
	return OCServAdvanced{
		TryMTUDiscovery:   true,
		IsolateWorkers:    true,
		DtlsLegacy:        true,
		Tcp:               true,
		Udp:               true,
		DenyRoaming:       false,
		CiscoClientCompat: true,
		Compression:       false,
		Keepalive:         true,
		DPD:               true,
		MobileDPD:         true,
		PredictableIPs:    false,
		PingLeases:        false,
		UseOcctl:          false,
		Rekey:             true,
		SwitchToTcp:       true,
		MaxSameClients:    2,
		KeepaliveSec:      32400,
		DPDSec:            90,
		MobileDPDSec:      1800,
		CookieTimeout:     300,
		RekeyTime:         172800,
		AuthTimeout:       240,
		SwitchToTcpTimeout: 25,
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
		// 旧版仅顶层 isolate_workers=true 时写入 JSON（omitempty）
		if legacyIsolate != nil && *legacyIsolate {
			dst.IsolateWorkers = true
		}
		return
	}
	if dst.MaxSameClients <= 0 {
		dst.MaxSameClients = def.MaxSameClients
	}
	if dst.KeepaliveSec <= 0 {
		dst.KeepaliveSec = def.KeepaliveSec
	}
	if dst.DPDSec <= 0 {
		dst.DPDSec = def.DPDSec
	}
	if dst.MobileDPDSec <= 0 {
		dst.MobileDPDSec = def.MobileDPDSec
	}
	if dst.CookieTimeout <= 0 {
		dst.CookieTimeout = def.CookieTimeout
	}
	if dst.RekeyTime <= 0 {
		dst.RekeyTime = def.RekeyTime
	}
	if dst.AuthTimeout <= 0 {
		dst.AuthTimeout = def.AuthTimeout
	}
	if dst.SwitchToTcpTimeout <= 0 {
		dst.SwitchToTcpTimeout = def.SwitchToTcpTimeout
	}
}
