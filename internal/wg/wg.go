package wg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
	pub, err := exec.Command("bash", "-c", fmt.Sprintf("echo %q | wg pubkey", privStr)).Output()
	if err != nil {
		return KeyPair{}, err
	}
	return KeyPair{Private: privStr, Public: strings.TrimSpace(string(pub))}, nil
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
	pub, err := exec.Command("bash", "-c", fmt.Sprintf("echo %q | wg pubkey", privStr)).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(pub)), nil
}

// RenderConf 生成 wg-quick 配置
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

// ClientConf 生成客户端配置（需 peer 含 private key）
func ClientConf(server store.WireGuardState, peer store.WGPeer, serverEndpoint string) string {
	var b bytes.Buffer
	b.WriteString("[Interface]\n")
	if peer.PrivateKey != "" {
		b.WriteString(fmt.Sprintf("PrivateKey = %s\n", peer.PrivateKey))
	}
	// 客户端地址取 allowed 中第一个 /32 或默认
	addr := "10.200.0.2/32"
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
	path := filepath.Join(confDir, iface+".conf")
	return os.WriteFile(path, []byte(RenderConf(wg)), 0600)
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
	var cmd *exec.Cmd
	if up {
		cmd = exec.Command("wg-quick", "up", iface)
	} else {
		cmd = exec.Command("wg-quick", "down", iface)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
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
