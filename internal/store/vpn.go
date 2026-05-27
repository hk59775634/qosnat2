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

// WireGuardState 单实例 WG 配置（嵌入 WireGuardInstance）
type WireGuardState struct {
	Enabled          bool     `json:"enabled"`
	Interface        string   `json:"interface"`
	ListenPort       int      `json:"listen_port"`
	Address          string   `json:"address"`
	PrivateKey       string   `json:"private_key"`
	PublicKey        string   `json:"public_key"`
	DNS              []string `json:"dns,omitempty"`
	ServerEndpoint   string   `json:"server_endpoint,omitempty"` // 客户端 Endpoint，如 203.0.113.1:51820
	Peers            []WGPeer `json:"peers"`
}

// WireGuardMode 实例角色：服务端接受客户端，或客户端连远端
type WireGuardMode string

const (
	WGModeServer WireGuardMode = "server"
	WGModeClient WireGuardMode = "client"
)

// WireGuardInstance 多实例之一（id 稳定主键；内嵌 WireGuardState，JSON 与旧版单层字段兼容）
type WireGuardInstance struct {
	ID   string        `json:"id"`
	Name string        `json:"name,omitempty"`
	Mode WireGuardMode `json:"mode"`
	WireGuardState
}

// WgPeerTrafficKey 流量统计存储键（避免不同实例下 peer 名冲突）
func WgPeerTrafficKey(instanceID, peerName string) string {
	return instanceID + "::" + peerName
}

// VPNState VPN 模块（WireGuard 多实例 + ocserv/OpenConnect）
type VPNState struct {
	WireGuards      []WireGuardInstance `json:"wireguards"`
	LegacyWireGuard *WireGuardState     `json:"wireguard,omitempty"` // 仅用于从旧 state 迁移
	OCServ          OCServState         `json:"ocserv"`
}
