package shaper

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/route"
)

// EDTConfig Per-IP EDT 数据面拓扑（无 IFB/HTB）
type EDTConfig struct {
	DevLAN     string
	FQFlows    int
	FQQuantum  int
	TxQueueLen int
}

// SetupEDT 在 LAN 上安装 fq 根 qdisc + clsact（Per-IP 限速由 BPF EDT/token bucket 完成）
func SetupEDT(cfg EDTConfig) error {
	if cfg.DevLAN == "" {
		return fmt.Errorf("DEV_LAN required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	_ = exec.CommandContext(ctx, "modprobe", "sch_fq").Run()
	cancel()
	if cfg.TxQueueLen > 0 {
		_ = exec.Command("ip", "link", "set", "dev", cfg.DevLAN, "txqueuelen", strconv.Itoa(cfg.TxQueueLen)).Run()
	}
	return SetupEDTDevice(cfg.DevLAN, cfg.FQFlows, cfg.FQQuantum)
}

// SetupEDTDevice 单接口：root fq + clsact
func SetupEDTDevice(dev string, fqFlows, fqQuantum int) error {
	if dev == "" {
		return fmt.Errorf("device required")
	}
	if !route.LinkExists(dev) {
		return fmt.Errorf("interface %s not found", dev)
	}
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "root").Run()
	args := edtRootFQArgs(dev, fqFlows, fqQuantum)
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			return fmt.Errorf("tc fq %s: %s %w", dev, msg, err)
		}
	}
	return ensureClsact(dev)
}

// edtRootFQArgs 构建 root fq qdisc 参数（EDT 需 plain fq，非 fq_codel）
func edtRootFQArgs(dev string, fqFlows, fqQuantum int) []string {
	args := []string{"tc", "qdisc", "add", "dev", dev, "root", "fq"}
	if fqFlows > 0 {
		args = append(args, "flows", strconv.Itoa(fqFlows))
	}
	if fqQuantum > 0 {
		args = append(args, "quantum", strconv.Itoa(fqQuantum))
	}
	return args
}

// TeardownEDT 清理 LAN 上 EDT 拓扑（不删 ifb0，兼容从 HTB 切换）
func TeardownEDT(devLAN string) {
	if devLAN != "" {
		TeardownDevice(devLAN)
	}
}
