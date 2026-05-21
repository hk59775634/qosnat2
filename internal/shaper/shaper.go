package shaper

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
)

// Config TC/IFB 拓扑参数
type Config struct {
	DevLAN    string
	Leaf      string // fq_codel | fq | cake
	FQFlows   int
	FQQuantum int
}

const IFBDev = "ifb0"

// SetupP0 创建 ifb0、HTB 根、clsact 占位（P2 前无 BPF attach）
func SetupP0(cfg Config) error {
	if cfg.DevLAN == "" {
		return fmt.Errorf("DEV_LAN required")
	}
	leaf := NormalizeLeaf(cfg.Leaf)
	mods := LeafModules(leaf)
	for _, m := range mods {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		_ = exec.CommandContext(ctx, "modprobe", m).Run()
		cancel()
	}
	if err := EnsureIFB(); err != nil {
		return err
	}
	fq := FQOpts{Flows: cfg.FQFlows, Quantum: cfg.FQQuantum}
	if err := setupHTBRoot(cfg.DevLAN, leaf, fq); err != nil {
		return err
	}
	if err := setupHTBRoot(IFBDev, leaf, fq); err != nil {
		return err
	}
	if err := ensureClsact(cfg.DevLAN); err != nil {
		return err
	}
	return nil
}

// EnsureIFB 确保 ifb0 存在（eBPF Load 与 TC 拓扑均依赖）
func EnsureIFB() error {
	return netif.EnsureIFB()
}

func setupHTBRoot(dev, leaf string, fq FQOpts) error {
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
	args := append([]string{"tc", "qdisc", "add", "dev", dev, "parent", "1:1"}, LeafTCArgs(leaf, fq)...)
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

// EnsureDevice HTB 根 + clsact（WireGuard 等附加接口）
func EnsureDevice(dev, leaf string) error {
	if dev == "" {
		return fmt.Errorf("device required")
	}
	if !route.LinkExists(dev) {
		return fmt.Errorf("interface %s not found", dev)
	}
	if leaf == "" {
		leaf = "fq_codel"
	}
	return EnsureDeviceWithFQ(dev, leaf, FQOpts{})
}

// EnsureDeviceWithFQ HTB 根 + clsact，可指定 fq 参数
func EnsureDeviceWithFQ(dev, leaf string, fq FQOpts) error {
	if dev == "" {
		return fmt.Errorf("device required")
	}
	if !route.LinkExists(dev) {
		return fmt.Errorf("interface %s not found", dev)
	}
	if leaf == "" {
		leaf = "fq_codel"
	}
	if err := setupHTBRoot(dev, leaf, fq); err != nil {
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
