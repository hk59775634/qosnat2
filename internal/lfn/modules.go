package lfn

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const modulesLoadPath = "/etc/modules-load.d/qosnat2-tcp-bbr.conf"

// EnsureBBR 加载 tcp_bbr 并写入 modules-load，开机可回放。
func EnsureBBR() error {
	if err := os.MkdirAll(filepath.Dir(modulesLoadPath), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(modulesLoadPath, []byte("tcp_bbr\n"), 0644); err != nil {
		return err
	}
	_ = exec.Command("modprobe", "tcp_bbr").Run()
	_ = exec.Command("modprobe", "sch_fq").Run()
	return nil
}

// RemoveBBRFile 最后一个长肥口关闭后去掉 modules-load（不 rmmod）。
func RemoveBBRFile() {
	_ = os.Remove(modulesLoadPath)
}

// ModulesLoadPath 供测试覆盖。
func ModulesLoadPath() string { return modulesLoadPath }

// FileHasBBR 检查 modules-load 内容。
func FileHasBBR(body string) bool {
	return strings.TrimSpace(body) == "tcp_bbr" || strings.Contains(body, "tcp_bbr\n")
}
