package netif

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const netplanBackupPath = "/var/lib/qosnat2/netplan-99.backup"

// BackupNetplanConfig 在 apply 前备份 99-qosnat2.yaml（不存在则 nil）
func BackupNetplanConfig() ([]byte, error) {
	b, err := os.ReadFile(NetplanConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	cp := append([]byte(nil), b...)
	if err := os.MkdirAll("/var/lib/qosnat2", 0750); err != nil {
		return cp, err
	}
	_ = os.WriteFile(netplanBackupPath, cp, 0644)
	return cp, nil
}

// RestoreNetplanConfig 恢复备份并 netplan apply
func RestoreNetplanConfig(backup []byte) error {
	if backup == nil {
		_ = os.Remove(NetplanConfigPath)
	} else {
		if err := os.MkdirAll("/etc/netplan", 0755); err != nil {
			return err
		}
		if err := os.WriteFile(NetplanConfigPath, backup, 0644); err != nil {
			return err
		}
	}
	return netplanGenerateApply()
}

func netplanGenerateApply() error {
	if _, err := exec.LookPath("netplan"); err != nil {
		return fmt.Errorf("netplan not installed")
	}
	if _, err := os.Stat(NetplanConfigPath); os.IsNotExist(err) {
		return nil
	}
	if out, err := exec.Command("netplan", "generate").CombinedOutput(); err != nil {
		return fmt.Errorf("netplan generate: %s %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.Command("netplan", "apply").CombinedOutput(); err != nil {
		return fmt.Errorf("netplan apply: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
