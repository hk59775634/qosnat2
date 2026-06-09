package netif

import (
	"fmt"
	"os/exec"
	"strings"
)

const IFBDev = "ifb0"

// EnsureIFB 创建 ifb0 并设置 txqueuelen（0 表示默认 5000）
func EnsureIFB() error {
	return EnsureIFBTuned(0)
}

// EnsureIFBUp 仅创建并拉起 ifb0，不修改 txqueuelen（eBPF Load 等早期阶段使用）
func EnsureIFBUp() error {
	if LinkExists(IFBDev) {
		if out, err := exec.Command("ip", "link", "set", IFBDev, "up").CombinedOutput(); err != nil {
			return fmt.Errorf("ip link set ifb0 up: %s %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	if out, err := exec.Command("ip", "link", "add", IFBDev, "type", "ifb").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link add ifb0: %s %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.Command("ip", "link", "set", IFBDev, "up").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set ifb0 up: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// EnsureIFBTuned 创建 ifb0 并按与物理网卡相同的规则设置 txqueuelen（0 表示默认 5000）
func EnsureIFBTuned(qlen int) error {
	if err := EnsureIFBUp(); err != nil {
		return err
	}
	return SetTxQueueLen(IFBDev, EffectiveTxQLen(qlen))
}
