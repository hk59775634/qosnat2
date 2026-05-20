package netif

import (
	"fmt"
	"os/exec"
	"strings"
)

// EnsureIFB 创建并拉起 ifb0（eBPF rewriteIFBIndex 与 TC 上行整形依赖）
func EnsureIFB() error {
	if LinkExists("ifb0") {
		if out, err := exec.Command("ip", "link", "set", "ifb0", "up").CombinedOutput(); err != nil {
			return fmt.Errorf("ip link set ifb0 up: %s %w", strings.TrimSpace(string(out)), err)
		}
		return nil
	}
	if out, err := exec.Command("ip", "link", "add", "ifb0", "type", "ifb").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link add ifb0: %s %w", strings.TrimSpace(string(out)), err)
	}
	if out, err := exec.Command("ip", "link", "set", "ifb0", "up").CombinedOutput(); err != nil {
		return fmt.Errorf("ip link set ifb0 up: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
