package store

// WGPeer WireGuard 对端
type WGPeer struct {
	Name                string    `json:"name"`
	PublicKey           string    `json:"public_key"`
	PrivateKey          string    `json:"private_key,omitempty"` // 客户端私钥（导出用，服务端 peer 通常无）
	AllowedIPs          []string  `json:"allowed_ips"`
	Endpoint            string    `json:"endpoint,omitempty"`
	PersistentKeepalive int       `json:"persistent_keepalive,omitempty"`
	PresharedKey        string    `json:"preshared_key,omitempty"`
	Rate                *HostRate `json:"rate,omitempty"` // 隧道 /32 限速（host_exact + HTB）
}

// WireGuardState 持久化 WG 配置
type WireGuardState struct {
	Enabled    bool     `json:"enabled"`
	Interface  string   `json:"interface"`
	ListenPort int      `json:"listen_port"`
	Address    string   `json:"address"`
	PrivateKey string   `json:"private_key"`
	PublicKey  string   `json:"public_key"`
	DNS             []string `json:"dns,omitempty"`
	ServerEndpoint  string   `json:"server_endpoint,omitempty"` // 客户端 Endpoint，如 203.0.113.1:51820
	Peers           []WGPeer `json:"peers"`
}

// VPNState VPN 模块（仅 WireGuard；不支持 IPsec / OpenVPN）
type VPNState struct {
	WireGuard WireGuardState `json:"wireguard"`
}
