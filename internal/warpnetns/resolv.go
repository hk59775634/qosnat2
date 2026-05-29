package warpnetns

import (
	"bytes"
	"os"
)

const (
	netnsResolvDir   = "/var/lib/qosnat2/warp-netns"
	netnsResolvFile  = "/var/lib/qosnat2/warp-netns/resolv.conf"
	hostResolvPath   = "/etc/resolv.conf"
	hostResolvBackup = "/var/lib/qosnat2/warp-host-resolv.bak"
)

func ensureNetnsResolvFile() {
	_ = os.MkdirAll(netnsResolvDir, 0755)
	if _, err := os.Stat(netnsResolvFile); err == nil {
		return
	}
	content := []byte("nameserver 1.1.1.1\nnameserver 1.0.0.1\n")
	if b, err := os.ReadFile(hostResolvBackup); err == nil && len(bytes.TrimSpace(b)) > 0 {
		content = b
	}
	_ = os.WriteFile(netnsResolvFile, content, 0644)
}

// BackupHostResolv 在首次启用 WARP 前保存宿主机 resolv.conf。
func BackupHostResolv() {
	if _, err := os.Stat(hostResolvBackup); err == nil {
		return
	}
	if b, err := os.ReadFile(hostResolvPath); err == nil && len(bytes.TrimSpace(b)) > 0 {
		_ = os.WriteFile(hostResolvBackup, b, 0644)
	}
}

// RestoreHostResolv 将宿主机 resolv.conf 恢复为 WARP 启用前的内容。
func RestoreHostResolv() {
	b, err := os.ReadFile(hostResolvBackup)
	if err != nil || len(bytes.TrimSpace(b)) == 0 {
		return
	}
	cur, _ := os.ReadFile(hostResolvPath)
	if bytes.Equal(bytes.TrimSpace(cur), bytes.TrimSpace(b)) {
		return
	}
	_ = os.WriteFile(hostResolvPath, b, 0644)
}

// warpSvcStartArgs 在 netns + 独立 mount ns 中启动 warp-svc，避免改写宿主机 resolv.conf。
func warpSvcStartArgs() []string {
	ensureNetnsResolvFile()
	BackupHostResolv()
	script := "mount --make-rprivate / 2>/dev/null || true; " +
		"mount --bind " + netnsResolvFile + " " + hostResolvPath + "; " +
		"exec " + warpSvc
	return []string{"netns", "exec", NetnsName, "unshare", "-m", "--", "bash", "-c", script}
}
