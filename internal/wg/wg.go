package wg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hk59775634/qosnat2/internal/linknet"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

const confDir = "/etc/wireguard"

// KeyPair 密钥对
type KeyPair struct {
	Private string `json:"private_key"`
	Public  string `json:"public_key"`
}

// Status 运行状态
type Status struct {
	Installed bool   `json:"installed"`
	Up        bool   `json:"up"`
	Raw       string `json:"raw,omitempty"`
}

func wgInstalled() bool {
	_, err := exec.LookPath("wg")
	return err == nil
}

// GenKeyPair wg genkey | wg pubkey
func GenKeyPair() (KeyPair, error) {
	if !wgInstalled() {
		return KeyPair{}, fmt.Errorf("wireguard-tools (wg) not installed")
	}
	priv, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return KeyPair{}, err
	}
	privStr := strings.TrimSpace(string(priv))
	pub, err := wgPubkeyFromPrivate(privStr)
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Private: privStr, Public: pub}, nil
}

// PublicKeyFromPrivate 由客户端私钥计算公钥（wg pubkey）
func PublicKeyFromPrivate(privateKey string) (string, error) {
	if !wgInstalled() {
		return "", fmt.Errorf("wireguard-tools (wg) not installed")
	}
	privStr := strings.TrimSpace(privateKey)
	if privStr == "" {
		return "", fmt.Errorf("private key empty")
	}
	return wgPubkeyFromPrivate(privStr)
}

func wgPubkeyFromPrivate(privStr string) (string, error) {
	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(privStr + "\n")
	pub, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(pub)), nil
}

// RenderConf 生成 wg-quick 配置。
// 若存在 RouteAllowedIPs=false 的 Peer，则 Table=off，仅为需要路由的 AllowedIPs 写 PostUp/PreDown。
func RenderConf(wg store.WireGuardState) string {
	var b bytes.Buffer
	iface := wg.Interface
	if iface == "" {
		iface = "wg0"
	}
	b.WriteString(fmt.Sprintf("[Interface]\nPrivateKey = %s\n", wg.PrivateKey))
	if wg.Address != "" {
		b.WriteString(fmt.Sprintf("Address = %s\n", wg.Address))
	}
	if wg.ListenPort > 0 {
		b.WriteString(fmt.Sprintf("ListenPort = %d\n", wg.ListenPort))
	}
	for _, d := range wg.DNS {
		b.WriteString(fmt.Sprintf("DNS = %s\n", d))
	}
	if peersNeedSelectiveRouting(wg.Peers) {
		b.WriteString("Table = off\n")
		writeAllowedIPRouteHooks(&b, wg.Peers)
	}
	for _, p := range wg.Peers {
		b.WriteString("\n[Peer]\n")
		b.WriteString(fmt.Sprintf("PublicKey = %s\n", p.PublicKey))
		if len(p.AllowedIPs) > 0 {
			b.WriteString(fmt.Sprintf("AllowedIPs = %s\n", strings.Join(p.AllowedIPs, ", ")))
		}
		if p.Endpoint != "" {
			b.WriteString(fmt.Sprintf("Endpoint = %s\n", p.Endpoint))
		}
		if p.PersistentKeepalive > 0 {
			b.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", p.PersistentKeepalive))
		}
		if p.PresharedKey != "" {
			b.WriteString(fmt.Sprintf("PresharedKey = %s\n", p.PresharedKey))
		}
	}
	return b.String()
}

func peersNeedSelectiveRouting(peers []store.WGPeer) bool {
	for _, p := range peers {
		if !store.PeerRouteAllowedIPs(p) {
			return true
		}
	}
	return false
}

func writeAllowedIPRouteHooks(b *bytes.Buffer, peers []store.WGPeer) {
	var ups, downs []string
	for _, p := range peers {
		if !store.PeerRouteAllowedIPs(p) {
			continue
		}
		for _, raw := range p.AllowedIPs {
			cidr, fam, ok := normalizeRouteCIDR(raw)
			if !ok {
				continue
			}
			ups = append(ups, fmt.Sprintf("ip %s route add %s dev %%i", fam, cidr))
			downs = append(downs, fmt.Sprintf("ip %s route del %s dev %%i", fam, cidr))
		}
	}
	if len(ups) == 0 {
		return
	}
	b.WriteString("PostUp = " + strings.Join(ups, "; ") + "\n")
	b.WriteString("PreDown = " + strings.Join(downs, "; ") + "\n")
}

// normalizeRouteCIDR 校验并规范路由 CIDR，返回 iproute fam（-4/-6）。
func normalizeRouteCIDR(s string) (cidr, fam string, ok bool) {
	s = strings.TrimSpace(s)
	if s == "" || strings.ContainsAny(s, "\n\r\t;|&`$\"'\\") {
		return "", "", false
	}
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return "", "", false
		}
		if v4 := ip.To4(); v4 != nil {
			return v4.String() + "/32", "-4", true
		}
		return ip.String() + "/128", "-6", true
	}
	ip, n, err := net.ParseCIDR(s)
	if err != nil {
		return "", "", false
	}
	ones, bits := n.Mask.Size()
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%s/%d", v4.String(), ones), "-4", true
	}
	_ = bits
	return fmt.Sprintf("%s/%d", ip.String(), ones), "-6", true
}

// ClientConf 生成客户端配置（需 peer 含 private key）
func ClientConf(server store.WireGuardState, peer store.WGPeer, serverEndpoint string) string {
	var b bytes.Buffer
	b.WriteString("[Interface]\n")
	if peer.PrivateKey != "" {
		b.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	}
	// 客户端地址取 allowed 中第一个含前缀的项，否则按服务端隧道网段回退 .2/32
	addr := linknet.WireGuardClientFallbackAddr(server.Address)
	for _, a := range peer.AllowedIPs {
		if strings.Contains(a, "/") {
			addr = a
			break
		}
	}
	b.WriteString(fmt.Sprintf("Address = %s\n", addr))
	for _, d := range server.DNS {
		b.WriteString(fmt.Sprintf("DNS = %s\n", d))
	}
	b.WriteString("\n[Peer]\n")
	b.WriteString(fmt.Sprintf("PublicKey = %s\n", server.PublicKey))
	if serverEndpoint != "" {
		b.WriteString(fmt.Sprintf("Endpoint = %s\n", serverEndpoint))
	}
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	if peer.PersistentKeepalive > 0 {
		b.WriteString(fmt.Sprintf("PersistentKeepalive = %d\n", peer.PersistentKeepalive))
	} else {
		b.WriteString("PersistentKeepalive = 25\n")
	}
	return b.String()
}

// WriteConf 写入 /etc/wireguard/{iface}.conf
func WriteConf(wg store.WireGuardState) error {
	if err := os.MkdirAll(confDir, 0700); err != nil {
		return err
	}
	iface := wg.Interface
	if iface == "" {
		iface = "wg0"
	}
	if err := netif.ValidateIfaceName(iface); err != nil {
		return err
	}
	path := filepath.Join(confDir, iface+".conf")
	return os.WriteFile(path, []byte(RenderConf(wg)), 0600)
}

func wgQuickDown(iface string) error {
	cmd := exec.Command("wg-quick", "down", iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		low := strings.ToLower(msg)
		if strings.Contains(low, "not found") || strings.Contains(low, "no such device") {
			return nil
		}
		return fmt.Errorf("wg-quick: %s %w", msg, err)
	}
	return nil
}

func wgQuickUp(iface string) error {
	cmd := exec.Command("wg-quick", "up", iface)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// normalizeAddressCIDR 将 Address 条目规范为 ip/prefix（裸 IP 按 /32 或 /128）
func normalizeAddressCIDR(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty address")
	}
	if !strings.Contains(s, "/") {
		ip := net.ParseIP(s)
		if ip == nil {
			return "", fmt.Errorf("invalid IP %q", s)
		}
		if ip.To4() != nil {
			return ip.To4().String() + "/32", nil
		}
		return ip.String() + "/128", nil
	}
	ip, n, err := net.ParseCIDR(s)
	if err != nil {
		return "", err
	}
	ones, _ := n.Mask.Size()
	if v4 := ip.To4(); v4 != nil {
		return fmt.Sprintf("%s/%d", v4.String(), ones), nil
	}
	return fmt.Sprintf("%s/%d", ip.String(), ones), nil
}

// parseWGAddressList 解析 wg-quick Address（逗号分隔）
func parseWGAddressList(addr string) ([]string, error) {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil, nil
	}
	parts := strings.Split(addr, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := normalizeAddressCIDR(p)
		if err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	sort.Strings(out)
	return out, nil
}

func cidrSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// liveGlobalCIDRs 读取接口上 global 地址（不含 link-local）
func liveGlobalCIDRs(iface string) ([]string, error) {
	out, err := exec.Command("ip", "-json", "addr", "show", "dev", iface).Output()
	if err != nil {
		return nil, fmt.Errorf("ip addr show %s: %w", iface, err)
	}
	var links []struct {
		AddrInfo []struct {
			Family    string `json:"family"`
			Local     string `json:"local"`
			Prefixlen int    `json:"prefixlen"`
			Scope     string `json:"scope"`
		} `json:"addr_info"`
	}
	if err := json.Unmarshal(out, &links); err != nil {
		return nil, err
	}
	var cidrs []string
	for _, link := range links {
		for _, a := range link.AddrInfo {
			if a.Family != "inet" && a.Family != "inet6" {
				continue
			}
			if a.Local == "" {
				continue
			}
			if a.Scope != "" && a.Scope != "global" {
				continue
			}
			n, err := normalizeAddressCIDR(fmt.Sprintf("%s/%d", a.Local, a.Prefixlen))
			if err != nil {
				continue
			}
			cidrs = append(cidrs, n)
		}
	}
	sort.Strings(cidrs)
	return cidrs, nil
}

// addressNeedsRecycle Address 与运行态不一致时需 down/up（syncconf 不处理 Address）
func addressNeedsRecycle(iface, desired string) bool {
	want, err := parseWGAddressList(desired)
	if err != nil {
		// 配置异常时保守走 recycle，避免静默保留旧地址
		return true
	}
	have, err := liveGlobalCIDRs(iface)
	if err != nil {
		return true
	}
	return !cidrSetsEqual(want, have)
}

// Apply up|down wg-quick
func Apply(wg store.WireGuardState, up bool) error {
	if !wgInstalled() {
		return fmt.Errorf("wireguard-tools not installed")
	}
	if err := WriteConf(wg); err != nil {
		return err
	}
	iface := wg.Interface
	if iface == "" {
		iface = "wg0"
	}
	if err := netif.ValidateIfaceName(iface); err != nil {
		return err
	}
	if !up {
		return wgQuickDown(iface)
	}
	// 已存在接口时：Address 等 wg-quick 字段只能靠 down/up；密钥/peer 可用 syncconf 热更新
	if netif.LinkExists(iface) {
		if addressNeedsRecycle(iface, wg.Address) {
			if err := wgQuickDown(iface); err != nil {
				return err
			}
			return wgQuickUp(iface)
		}
		stripped, err := exec.Command("wg-quick", "strip", iface).Output()
		if err != nil {
			return fmt.Errorf("wg-quick strip %s: %w", iface, err)
		}
		cmd := exec.Command("wg", "syncconf", iface, "/proc/self/fd/0")
		cmd.Stdin = bytes.NewReader(stripped)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("wg syncconf: %s %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	return wgQuickUp(iface)
}

// ShowStatus wg show
func ShowStatus(iface string) Status {
	st := Status{Installed: wgInstalled()}
	if !st.Installed {
		return st
	}
	args := []string{"show"}
	if iface != "" {
		args = append(args, iface)
	}
	out, err := exec.Command("wg", args...).CombinedOutput()
	if err == nil && len(out) > 0 {
		st.Up = true
		st.Raw = string(out)
	}
	return st
}
