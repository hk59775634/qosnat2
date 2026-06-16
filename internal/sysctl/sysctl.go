package sysctl

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Defaults P0 内核参数（§15）
var Defaults = map[string]string{
	"net.ipv4.ip_forward":              "1",
	"net.ipv4.conf.all.rp_filter":      "0",
	"net.core.rmem_max":                "134217728",
	"net.core.wmem_max":                "134217728",
	"net.netfilter.nf_conntrack_max":   "2097152",
}

const confPath = "/etc/sysctl.d/99-qosnat2.conf"

// ValidateValue rejects sysctl values that could break the generated config file.
func ValidateValue(v string) error {
	if strings.ContainsAny(v, "\n\r\x00=") {
		return fmt.Errorf("invalid sysctl value")
	}
	return nil
}

// Apply 写入 sysctl.d 并 sysctl -p（extra 为用户覆盖，usePerformance 合并高性能预设）
func Apply(extra map[string]string, usePerformance bool) error {
	merged := Merge(extra, usePerformance)
	for _, v := range merged {
		if err := ValidateValue(v); err != nil {
			return err
		}
	}
	var b strings.Builder
	b.WriteString("# qosnat2 — generated\n")
	for k, v := range merged {
		b.WriteString(fmt.Sprintf("%s = %s\n", k, v))
	}
	if err := os.MkdirAll(filepath.Dir(confPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(confPath, []byte(b.String()), 0644); err != nil {
		return err
	}
	// -p 仅加载本文件，避免 sysctl --system 扫描全部配置时偶发长时间阻塞
	out, err := exec.Command("sysctl", "-p", confPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysctl -p %s: %s %w", confPath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ApplyFast 仅 -w 热应用（启动时）
func ApplyFast(extra map[string]string, usePerformance bool) {
	for k, v := range Merge(extra, usePerformance) {
		_ = exec.Command("sysctl", "-w", k+"="+v).Run()
	}
}
