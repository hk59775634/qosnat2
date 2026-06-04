package dnsmasq

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	// ChnroutesLibPath release 与本地缓存的预编译 dnsmasq（Ubuntu 24.04 amd64）。
	ChnroutesLibPath     = "/usr/local/lib/qosnat2/dnsmasq-chnroutes"
	SystemDnsmasqPath    = "/usr/sbin/dnsmasq"
	ReleaseTarDnsmasqRel = "lib/dnsmasq-chnroutes"
)

// PrebuiltCandidatePaths 查找预编译 dnsmasq 的候选路径（按优先级）。
func PrebuiltCandidatePaths() []string {
	var out []string
	seen := map[string]bool{}
	add := func(p string) {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			return
		}
		seen[p] = true
		out = append(out, p)
	}
	add(ChnroutesLibPath)
	for _, root := range []string{
		os.Getenv("QOSNAT_ROOT"),
		"/opt/qosnat2",
	} {
		root = strings.TrimSpace(root)
		if root == "" {
			continue
		}
		add(filepath.Join(root, "dist", "lib", "dnsmasq-chnroutes"))
	}
	if wr := strings.TrimSpace(os.Getenv("WEB_ROOT")); wr != "" {
		add(filepath.Clean(filepath.Join(wr, "..", "..", "dist", "lib", "dnsmasq-chnroutes")))
	}
	return out
}

// BinarySupportsChnroutes 检测指定路径的二进制是否含 chnroutes 补丁。
func BinarySupportsChnroutes(path string) bool {
	out, err := exec.Command(path, "--help").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), "chnroutes-file")
}

// LocatePrebuiltChnroutes 返回第一个存在且支持 chnroutes 的预编译路径。
func LocatePrebuiltChnroutes() (string, error) {
	for _, p := range PrebuiltCandidatePaths() {
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if BinarySupportsChnroutes(p) {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到预编译 dnsmasq-chnroutes（请升级 release 或运行 build-dnsmasq-chnroutes.sh）")
}

// InstallChnroutesBinary 将预编译二进制安装到 ChnroutesLibPath 与 SystemDnsmasqPath。
func InstallChnroutesBinary(src string) error {
	src = filepath.Clean(src)
	st, err := os.Stat(src)
	if err != nil {
		return err
	}
	if st.IsDir() {
		return fmt.Errorf("not a file: %s", src)
	}
	if !BinarySupportsChnroutes(src) {
		return fmt.Errorf("binary lacks chnroutes patch: %s", src)
	}
	if err := os.MkdirAll(filepath.Dir(ChnroutesLibPath), 0755); err != nil {
		return err
	}
	if err := copyExecutable(src, ChnroutesLibPath); err != nil {
		return fmt.Errorf("install %s: %w", ChnroutesLibPath, err)
	}
	if err := installSystemDnsmasq(src); err != nil {
		return err
	}
	if active, _ := exec.Command("systemctl", "is-active", "dnsmasq").Output(); strings.TrimSpace(string(active)) == "active" {
		_ = exec.Command("systemctl", "restart", "dnsmasq").Run()
	}
	return nil
}

func installSystemDnsmasq(src string) error {
	if st, err := os.Stat(SystemDnsmasqPath); err == nil && !st.IsDir() {
		backup := SystemDnsmasqPath + ".dist"
		if _, err := os.Stat(backup); os.IsNotExist(err) {
			if err := os.Rename(SystemDnsmasqPath, backup); err != nil {
				return fmt.Errorf("backup %s: %w", SystemDnsmasqPath, err)
			}
		}
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}
	return copyExecutable(src, SystemDnsmasqPath)
}

func copyExecutable(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := out.ReadFrom(in); err != nil {
		return err
	}
	return out.Chmod(0755)
}
