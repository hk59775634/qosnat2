// Package singbox 管理 sing-box 二进制与 ProxyEgress TUN 实例（auto_route=false，由策略路由接入）。
package singbox

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	// BinaryPath 安装目标路径。
	BinaryPath = "/usr/local/bin/sing-box"
	// Version 固定版本，避免静默升级破坏配置兼容性。
	Version = "1.11.15"
	// DataDir 配置与 PID 目录。
	DataDir = "/var/lib/qosnat2/proxy-egress"
	// InstallStatusFile 安装任务状态。
	InstallStatusFile = "/var/lib/qosnat2/sing-box-install-status.json"
	// InstallLogFile 安装日志。
	InstallLogFile = "/var/lib/qosnat2/sing-box-install.log"
)

var (
	instMu sync.Mutex
	procs  = map[string]*os.Process{} // proxyID → process
)

// Installed 是否已安装可用二进制。
func Installed() bool {
	st, err := os.Stat(BinaryPath)
	if err != nil || st.IsDir() {
		return false
	}
	out, err := exec.Command(BinaryPath, "version").CombinedOutput()
	return err == nil && strings.Contains(string(out), "sing-box")
}

// VersionString 返回 `sing-box version` 输出首行。
func VersionString() string {
	out, err := exec.Command(BinaryPath, "version").CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func instanceDir(id string) string {
	return filepath.Join(DataDir, sanitizeID(id))
}

func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			b.WriteRune(r)
		}
	}
	out := b.String()
	if out == "" {
		return "unknown"
	}
	return out
}

func configPath(id string) string  { return filepath.Join(instanceDir(id), "config.json") }
func pidPath(id string) string     { return filepath.Join(instanceDir(id), "sing-box.pid") }
func logPath(id string) string     { return filepath.Join(instanceDir(id), "sing-box.log") }

// BuildConfig 生成 sing-box 配置（TUN inbound + 代理 outbound，不劫持主表路由）。
func BuildConfig(p store.ProxyEgress) (map[string]any, error) {
	if err := store.NormalizeProxyEgress(&p); err != nil {
		return nil, err
	}
	if p.TunIndex < 0 {
		return nil, fmt.Errorf("tun_index not allocated")
	}
	dev := store.ProxyTunDevice(p.TunIndex)
	addr := store.ProxyTunAddress(p.TunIndex)
	if dev == "" || addr == "" {
		return nil, fmt.Errorf("invalid tun index")
	}

	outbound := map[string]any{
		"type":        outboundType(p.Type),
		"tag":         "proxy",
		"server":      p.Server,
		"server_port": p.Port,
	}
	if p.Username != "" {
		outbound["username"] = p.Username
	}
	if p.Password != "" && p.Password != "***" {
		outbound["password"] = p.Password
	}
	if p.Type == "https" {
		outbound["tls"] = map[string]any{"enabled": true}
	}

	return map[string]any{
		"log": map[string]any{
			"level":     "warn",
			"timestamp": true,
		},
		"inbounds": []any{
			map[string]any{
				"type":                 "tun",
				"tag":                  "tun-in",
				"interface_name":       dev,
				"inet4_address":        addr,
				"mtu":                  1500,
				"auto_route":           false,
				"strict_route":         false,
				"stack":                "system",
				"sniff":                true,
				"sniff_override_destination": true,
			},
		},
		"outbounds": []any{
			outbound,
			map[string]any{"type": "direct", "tag": "direct"},
			map[string]any{"type": "block", "tag": "block"},
		},
		"route": map[string]any{
			"final":                 "proxy",
			"auto_detect_interface": true,
		},
	}, nil
}

func outboundType(t string) string {
	switch strings.ToLower(t) {
	case "socks5", "socks":
		return "socks"
	case "http", "https":
		return "http"
	default:
		return "socks"
	}
}

// WriteConfig 写入实例配置。
func WriteConfig(p store.ProxyEgress) error {
	cfg, err := BuildConfig(p)
	if err != nil {
		return err
	}
	dir := instanceDir(p.ID)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(p.ID), b, 0600)
}

// IsRunning 进程是否在跑且 TUN 存在。
func IsRunning(p store.ProxyEgress) bool {
	if !pidAlive(p.ID) {
		return false
	}
	dev := store.ProxyTunDevice(p.TunIndex)
	if dev == "" {
		return false
	}
	return linkExists(dev)
}

func linkExists(name string) bool {
	out, err := exec.Command("ip", "link", "show", "dev", name).CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func pidAlive(id string) bool {
	b, err := os.ReadFile(pidPath(id))
	if err != nil {
		return false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 0 {
		return false
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return proc.Signal(syscall.Signal(0)) == nil
}

// Start 启动 sing-box 实例（幂等：已运行则跳过）。
func Start(p store.ProxyEgress) error {
	instMu.Lock()
	defer instMu.Unlock()
	if IsRunning(p) {
		return nil
	}
	_ = StopLocked(p.ID)
	if err := WriteConfig(p); err != nil {
		return err
	}
	if !Installed() {
		return fmt.Errorf("sing-box not installed")
	}
	logF, err := os.OpenFile(logPath(p.ID), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	cmd := exec.Command(BinaryPath, "run", "-c", configPath(p.ID))
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = logF.Close()
		return fmt.Errorf("start sing-box: %w", err)
	}
	procs[p.ID] = cmd.Process
	_ = os.WriteFile(pidPath(p.ID), []byte(strconv.Itoa(cmd.Process.Pid)), 0600)
	go func() {
		_ = cmd.Wait()
		_ = logF.Close()
		instMu.Lock()
		if procs[p.ID] == cmd.Process {
			delete(procs, p.ID)
		}
		instMu.Unlock()
	}()
	deadline := time.Now().Add(8 * time.Second)
	dev := store.ProxyTunDevice(p.TunIndex)
	for time.Now().Before(deadline) {
		if linkExists(dev) && pidAlive(p.ID) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	_ = StopLocked(p.ID)
	tail := readLogTail(p.ID, 2<<10)
	if tail != "" {
		return fmt.Errorf("sing-box failed to create TUN %s: %s", dev, tail)
	}
	return fmt.Errorf("sing-box failed to create TUN %s", dev)
}

// Stop 停止实例。
func Stop(id string) error {
	instMu.Lock()
	defer instMu.Unlock()
	return StopLocked(id)
}

// StopLocked 调用方已持有 instMu。
func StopLocked(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	if proc, ok := procs[id]; ok && proc != nil {
		_ = syscall.Kill(-proc.Pid, syscall.SIGTERM)
		delete(procs, id)
	}
	b, err := os.ReadFile(pidPath(id))
	if err == nil {
		if pid, e := strconv.Atoi(strings.TrimSpace(string(b))); e == nil && pid > 0 {
			_ = syscall.Kill(-pid, syscall.SIGTERM)
			time.Sleep(300 * time.Millisecond)
			_ = syscall.Kill(pid, syscall.SIGKILL)
			_ = syscall.Kill(-pid, syscall.SIGKILL)
		}
	}
	_ = os.Remove(pidPath(id))
	return nil
}

// StopAll 停止全部已知实例。
func StopAll(list []store.ProxyEgress) {
	for _, p := range list {
		_ = Stop(p.ID)
	}
}

func readLogTail(id string, max int) string {
	b, err := os.ReadFile(logPath(id))
	if err != nil {
		return ""
	}
	if len(b) > max {
		b = b[len(b)-max:]
	}
	return strings.TrimSpace(string(b))
}

// ProbeEgressIP 经 TUN 探测出口公网 IP（best-effort）。
func ProbeEgressIP(p store.ProxyEgress) string {
	dev := store.ProxyTunDevice(p.TunIndex)
	if dev == "" || !linkExists(dev) {
		return ""
	}
	cmd := exec.Command("curl", "-fsS", "--max-time", "8", "--interface", dev, "https://ifconfig.me/ip")
	out, err := cmd.CombinedOutput()
	if err != nil {
		cmd = exec.Command("curl", "-fsS", "--max-time", "8", "--interface", dev, "https://api.ipify.org")
		out, err = cmd.CombinedOutput()
		if err != nil {
			return ""
		}
	}
	ip := strings.TrimSpace(string(out))
	if netIP := parseIPv4(ip); netIP == "" {
		return ""
	}
	return ip
}

func parseIPv4(s string) string {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return ""
	}
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 || n > 255 {
			return ""
		}
	}
	return s
}

// DownloadAndInstall 从 GitHub Release 安装固定版本（需 root）。
func DownloadAndInstall(logW io.Writer) error {
	if logW == nil {
		logW = io.Discard
	}
	arch := runtime.GOARCH
	switch arch {
	case "amd64", "arm64":
	default:
		return fmt.Errorf("unsupported arch %s", arch)
	}
	name := fmt.Sprintf("sing-box-%s-linux-%s", Version, arch)
	url := fmt.Sprintf("https://github.com/SagerNet/sing-box/releases/download/v%s/%s.tar.gz", Version, name)
	fmt.Fprintf(logW, "download %s\n", url)
	tmpDir, err := os.MkdirTemp("", "sing-box-install-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)
	tgz := filepath.Join(tmpDir, "sing-box.tar.gz")
	if err := httpDownload(url, tgz); err != nil {
		return err
	}
	fmt.Fprintf(logW, "extract\n")
	out, err := exec.Command("tar", "-xzf", tgz, "-C", tmpDir).CombinedOutput()
	if err != nil {
		return fmt.Errorf("tar: %s %w", strings.TrimSpace(string(out)), err)
	}
	bin := filepath.Join(tmpDir, name, "sing-box")
	if _, err := os.Stat(bin); err != nil {
		return fmt.Errorf("binary missing in archive: %w", err)
	}
	fmt.Fprintf(logW, "install %s\n", BinaryPath)
	data, err := os.ReadFile(bin)
	if err != nil {
		return err
	}
	tmpBin := BinaryPath + ".tmp"
	if err := os.WriteFile(tmpBin, data, 0755); err != nil {
		return err
	}
	if err := os.Rename(tmpBin, BinaryPath); err != nil {
		_ = os.Remove(tmpBin)
		return err
	}
	fmt.Fprintf(logW, "ok %s\n", VersionString())
	return nil
}

func httpDownload(url, dest string) error {
	client := &http.Client{Timeout: 3 * time.Minute}
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "qosnat2-sing-box-installer")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}
