package ocserv

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const ocservSystemdUnitPath = "/etc/systemd/system/ocserv.service"

// EnsureSystemdUnit 若 ocserv 已安装但缺少 systemd 单元则写入默认 standalone 单元。
func EnsureSystemdUnit() error {
	if _, err := os.Stat(ocservSystemdUnitPath); err == nil {
		return nil
	}
	st := InstallInfo()
	if !st.Installed {
		return fmt.Errorf("ocserv not installed")
	}
	bin := strings.TrimSpace(st.Binary)
	if bin == "" {
		bin = BinaryPath
	}
	body := systemdUnitContent(bin, SysconfDir)
	if err := os.WriteFile(ocservSystemdUnitPath, []byte(body), 0644); err != nil {
		return fmt.Errorf("write systemd unit: %w", err)
	}
	out, err := exec.Command("systemctl", "daemon-reload").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

func systemdUnitContent(binary, sysconfDir string) string {
	return fmt.Sprintf(`[Unit]
Description=OpenConnect SSL VPN server
Documentation=man:ocserv(8)
After=network-online.target

[Service]
PrivateTmp=true
PIDFile=/run/ocserv.pid
Type=simple
ExecStart=%s --log-stderr --foreground --pid-file /run/ocserv.pid --config %s/ocserv.conf
ExecReload=/bin/kill -HUP $MAINPID

[Install]
WantedBy=multi-user.target
`, binary, sysconfDir)
}
