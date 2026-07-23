package wg

import (
	"fmt"
	"net"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// ParseEndpointHost 从 host:port / [v6]:port / host 解析主机部分。
func ParseEndpointHost(endpoint string) string {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(endpoint)
	if err == nil {
		return strings.Trim(host, "[]")
	}
	// 无端口或纯主机
	if strings.HasPrefix(endpoint, "[") {
		if i := strings.Index(endpoint, "]"); i > 1 {
			return endpoint[1:i]
		}
	}
	if i := strings.LastIndex(endpoint, ":"); i > 0 && strings.Count(endpoint, ":") == 1 {
		return endpoint[:i]
	}
	return endpoint
}

// EndpointLooksTunnelPinned 运行时 endpoint 主机落在本 Peer AllowedIPs 内时，视为错误漫游到隧道地址。
func EndpointLooksTunnelPinned(runtimeEndpoint string, allowedIPs []string) bool {
	host := ParseEndpointHost(runtimeEndpoint)
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, raw := range allowedIPs {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if !strings.Contains(raw, "/") {
			if p := net.ParseIP(raw); p != nil && p.Equal(ip) {
				return true
			}
			continue
		}
		_, n, err := net.ParseCIDR(raw)
		if err != nil {
			continue
		}
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// EndpointsEqual 比较配置与运行时 endpoint（忽略空白；主机大小写不敏感）。
func EndpointsEqual(configEP, runtimeEP string) bool {
	c := strings.TrimSpace(configEP)
	r := strings.TrimSpace(runtimeEP)
	if c == "" && r == "" {
		return true
	}
	if c == "" || r == "" {
		return false
	}
	ch, cp, err1 := net.SplitHostPort(c)
	rh, rp, err2 := net.SplitHostPort(r)
	if err1 == nil && err2 == nil {
		return strings.EqualFold(strings.Trim(ch, "[]"), strings.Trim(rh, "[]")) && cp == rp
	}
	return strings.EqualFold(c, r)
}

// ShouldRepinEndpoint 配置了静态 Endpoint，且运行时漂移（尤其漫游到 AllowedIPs/隧道地址）时需要钉回。
func ShouldRepinEndpoint(configEP, runtimeEP string, allowedIPs []string) bool {
	configEP = strings.TrimSpace(configEP)
	if configEP == "" {
		return false
	}
	if EndpointsEqual(configEP, runtimeEP) {
		return false
	}
	if strings.TrimSpace(runtimeEP) == "" {
		return true
	}
	// 明确错误：运行时落到本 Peer 的隧道/AllowedIPs 地址（如 100.64.1.2:5060）
	if EndpointLooksTunnelPinned(runtimeEP, allowedIPs) {
		return true
	}
	// 站点静态 Endpoint：配置写死后一律钉回，避免经其它 Peer（0.0.0.0/0）回环后漫游走偏
	return true
}

// SetPeerEndpoint 将运行时 peer endpoint 设为指定值。
func SetPeerEndpoint(iface, publicKey, endpoint string) error {
	iface = strings.TrimSpace(iface)
	if iface == "" {
		iface = "wg0"
	}
	publicKey = strings.TrimSpace(publicKey)
	endpoint = strings.TrimSpace(endpoint)
	if publicKey == "" || endpoint == "" {
		return fmt.Errorf("peer public key and endpoint required")
	}
	out, err := exec.Command("wg", "set", iface, "peer", publicKey, "endpoint", endpoint).CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg set peer endpoint: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// PinConfiguredEndpoints 将配置中写死的 Endpoint 钉回运行态；返回实际纠正的 peer 公钥列表。
func PinConfiguredEndpoints(iface string, peers []store.WGPeer) ([]string, error) {
	if !wgInstalled() {
		return nil, nil
	}
	iface = strings.TrimSpace(iface)
	if iface == "" {
		iface = "wg0"
	}
	live, err := DumpPeerStats(iface)
	if err != nil {
		return nil, err
	}
	var fixed []string
	for _, p := range peers {
		want := strings.TrimSpace(p.Endpoint)
		if want == "" || strings.TrimSpace(p.PublicKey) == "" {
			continue
		}
		st, ok := live[p.PublicKey]
		runtimeEP := ""
		if ok {
			runtimeEP = st.Endpoint
		}
		if !ShouldRepinEndpoint(want, runtimeEP, p.AllowedIPs) {
			continue
		}
		if err := SetPeerEndpoint(iface, p.PublicKey, want); err != nil {
			return fixed, err
		}
		fixed = append(fixed, p.PublicKey)
	}
	return fixed, nil
}
