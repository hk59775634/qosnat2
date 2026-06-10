package frr

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	FRRConfPath    = "/etc/frr/frr.conf"
	DaemonsPath    = "/etc/frr/daemons"
	PackageName    = "frr"
)

var editableConfigPaths = map[string]string{
	"frr.conf": FRRConfPath,
	"extra":    ExtraConfig,
	"managed":  ManagedRoutes,
	"dynamic":  DynamicRoutingConfig,
	"daemons":  DaemonsPath,
	"include":  IncludeSnippet,
}

// PackageInstalled 是否已安装 frr 软件包（vtysh 存在）。
func PackageInstalled() bool {
	_, err := exec.LookPath("vtysh")
	return err == nil
}

// ServiceActive frr systemd 单元是否 active。
func ServiceActive() bool {
	out, err := exec.Command("systemctl", "is-active", "frr").CombinedOutput()
	return err == nil && strings.TrimSpace(string(out)) == "active"
}

// BootEnabled 是否已设置开机启动（systemd enabled）。
func BootEnabled() bool {
	out, err := exec.Command("systemctl", "is-enabled", "frr").CombinedOutput()
	s := strings.TrimSpace(string(out))
	return err == nil && (s == "enabled" || s == "enabled-runtime")
}

// Status 返回 FRR 运行状态摘要。
func Status() map[string]any {
	st := map[string]any{
		"package_installed": PackageInstalled(),
		"active":            ServiceActive(),
		"boot_enabled":      BootEnabled(),
		"version":           "",
	}
	if !PackageInstalled() {
		return st
	}
	if out, err := exec.Command("vtysh", "-c", "show version").CombinedOutput(); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.Contains(strings.ToLower(line), "frr version") {
				st["version"] = strings.TrimSpace(line)
				break
			}
		}
	}
	return st
}

// SetBootEnabled 设置 frr 是否开机启动。
func SetBootEnabled(on bool) error {
	if !PackageInstalled() {
		return fmt.Errorf("frr not installed")
	}
	var cmd *exec.Cmd
	if on {
		cmd = exec.Command("systemctl", "enable", "frr")
	} else {
		cmd = exec.Command("systemctl", "disable", "frr")
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ServiceAction 执行 start|stop|restart。
func ServiceAction(action string) error {
	if !PackageInstalled() {
		return fmt.Errorf("frr not installed")
	}
	switch action {
	case "start", "stop", "restart":
	default:
		return fmt.Errorf("invalid action: %s", action)
	}
	out, err := exec.Command("systemctl", action, "frr").CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl %s frr: %s %w", action, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ReadConfigFile 读取可编辑配置文件。
func ReadConfigFile(which string) (string, string, error) {
	path, err := resolveConfigPath(which)
	if err != nil {
		return "", "", err
	}
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return path, "", nil
	}
	if err != nil {
		return path, "", err
	}
	return path, string(b), nil
}

// WriteConfigFile 写入可编辑配置文件。
func WriteConfigFile(which, body string) (string, error) {
	path, err := resolveConfigPath(which)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		return "", err
	}
	if which == "extra" {
		_ = ensureInclude()
	}
	return path, nil
}

func resolveConfigPath(which string) (string, error) {
	which = strings.TrimSpace(which)
	p, ok := editableConfigPaths[which]
	if !ok {
		return "", fmt.Errorf("unknown config: %s", which)
	}
	clean := filepath.Clean(p)
	if !strings.HasPrefix(clean, "/etc/frr/") && !strings.HasPrefix(clean, "/etc/qosnat2/frr/") {
		return "", fmt.Errorf("path not allowed")
	}
	return clean, nil
}

// InstallScriptPath 返回 install-frr.sh 路径。
func InstallScriptPath() string {
	for _, root := range []string{os.Getenv("QOSNAT_ROOT"), "/opt/qosnat2"} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		p := filepath.Join(root, "scripts", "install-frr.sh")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p
		}
	}
	return "/opt/qosnat2/scripts/install-frr.sh"
}
