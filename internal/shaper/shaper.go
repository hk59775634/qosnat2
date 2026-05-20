package shaper

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// Config TC/IFB 拓扑参数
type Config struct {
	DevLAN string
	Leaf   string // fq_codel | fq
}

const IFBDev = "ifb0"

// SetupP0 创建 ifb0、HTB 根、clsact 占位（P2 前无 BPF attach）
func SetupP0(cfg Config) error {
	if cfg.DevLAN == "" {
		return fmt.Errorf("DEV_LAN required")
	}
	leaf := cfg.Leaf
	if leaf == "" {
		leaf = "fq_codel"
	}
	mods := []string{"ifb", "sch_htb", "sch_fq_codel", "sch_fq", "cls_bpf", "act_bpf", "act_mirred"}
	for _, m := range mods {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = exec.CommandContext(ctx, "modprobe", m).Run()
		cancel()
	}
	if err := ensureIFB(); err != nil {
		return err
	}
	if err := setupHTBRoot(cfg.DevLAN, leaf); err != nil {
		return err
	}
	if err := setupHTBRoot(IFBDev, leaf); err != nil {
		return err
	}
	if err := ensureClsact(cfg.DevLAN); err != nil {
		return err
	}
	return nil
}

func ensureIFB() error {
	if linkExists(IFBDev) {
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

func setupHTBRoot(dev, leaf string) error {
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "root").Run()
	if out, err := exec.Command("tc", "qdisc", "add", "dev", dev, "root", "handle", "1:", "htb", "default", "1").CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("tc htb root %s: %s %w", dev, strings.TrimSpace(string(out)), err)
		}
	}
	if out, err := exec.Command("tc", "class", "add", "dev", dev, "parent", "1:", "classid", "1:1", "htb", "rate", "10gbit", "ceil", "10gbit").CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("tc class 1:1 %s: %s %w", dev, strings.TrimSpace(string(out)), err)
		}
	}
	_ = exec.Command("tc", "qdisc", "del", "dev", dev, "parent", "1:1").Run()
	args := []string{"tc", "qdisc", "add", "dev", dev, "parent", "1:1", leaf}
	if leaf == "fq_codel" {
		args = append(args, "limit", "10240")
	}
	if out, err := exec.Command(args[0], args[1:]...).CombinedOutput(); err != nil {
		if !strings.Contains(string(out), "File exists") {
			return fmt.Errorf("tc leaf %s on %s: %s %w", leaf, dev, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func ensureClsact(dev string) error {
	if out, err := exec.Command("tc", "qdisc", "add", "dev", dev, "clsact").CombinedOutput(); err != nil {
		msg := string(out)
		if strings.Contains(msg, "File exists") || strings.Contains(msg, "Exclusivity flag on") {
			return nil
		}
		return fmt.Errorf("tc clsact %s: %s %w", dev, strings.TrimSpace(msg), err)
	}
	return nil
}

func linkExists(name string) bool {
	_, err := exec.Command("ip", "link", "show", name).Output()
	return err == nil
}

// EnsureDevice HTB 根 + clsact（WireGuard 等附加接口）
func EnsureDevice(dev, leaf string) error {
	if dev == "" {
		return fmt.Errorf("device required")
	}
	if !linkExists(dev) {
		return fmt.Errorf("interface %s not found", dev)
	}
	if leaf == "" {
		leaf = "fq_codel"
	}
	if err := setupHTBRoot(dev, leaf); err != nil {
		return err
	}
	return ensureClsact(dev)
}

// Teardown 停止时清理（可选）
func Teardown(devLAN string) {
	if devLAN != "" {
		_ = exec.Command("tc", "qdisc", "del", "dev", devLAN, "clsact").Run()
		_ = exec.Command("tc", "qdisc", "del", "dev", devLAN, "root").Run()
	}
	_ = exec.Command("tc", "qdisc", "del", "dev", IFBDev, "root").Run()
	_ = exec.Command("ip", "link", "del", IFBDev).Run()
}
