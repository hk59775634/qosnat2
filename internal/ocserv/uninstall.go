package ocserv

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const ocservSystemdUnit = "/etc/systemd/system/ocserv.service"

// UninstallBinaries 停止 ocserv、移除 systemd 单元与 /usr/local 下的可执行文件。
// 不删除 /etc/ocserv 配置目录。
func UninstallBinaries() error {
	_, _ = exec.Command("systemctl", "stop", "ocserv").CombinedOutput()
	_, _ = exec.Command("systemctl", "disable", "ocserv").CombinedOutput()

	_ = os.Remove("/etc/systemd/system/multi-user.target.wants/ocserv.service")
	if err := os.Remove(ocservSystemdUnit); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove systemd unit: %w", err)
	}
	if out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s %w", strings.TrimSpace(string(out)), err)
	}

	candidates := []string{
		BinaryPath,
		OcpasswdPath,
		OcctlPath,
		filepath.Join(filepath.Dir(BinaryPath), "ocpasswd"),
		"/usr/local/bin/ocpasswd",
		"/usr/local/sbin/ocpasswd",
	}
	if p, err := exec.LookPath("ocserv"); err == nil && strings.HasPrefix(p, "/usr/local/") {
		candidates = append(candidates, p)
	}
	if p, err := exec.LookPath("occtl"); err == nil && strings.HasPrefix(p, "/usr/local/") {
		candidates = append(candidates, p)
	}
	if p, err := exec.LookPath("ocpasswd"); err == nil && strings.HasPrefix(p, "/usr/local/") {
		candidates = append(candidates, p)
	}

	seen := map[string]struct{}{}
	for _, p := range candidates {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", p, err)
		}
	}
	_ = os.Remove(ocservReleaseTagFile)
	return nil
}

// UninstallFromSourceInstall 保留旧名称以兼容调用方。
func UninstallFromSourceInstall() error { return UninstallBinaries() }
