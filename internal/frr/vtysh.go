package frr

import (
	"fmt"
	"os/exec"
	"strings"
)

// runVTYSH 通过 stdin 执行 vtysh 配置命令（configure terminal / write memory 等）。
// 注意：vtysh -f 仅接受 frr.conf 原生语法，不能用于交互式命令脚本。
func runVTYSH(script string) error {
	script = strings.TrimSpace(script)
	if script == "" {
		return nil
	}
	cmd := exec.Command("vtysh")
	cmd.Stdin = strings.NewReader(script + "\n")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
