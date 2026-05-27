package jool

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

const instanceName = "qosnat2"

// Installed reports whether jool CLI is available
func Installed() bool {
	_, err := exec.LookPath("jool")
	return err == nil
}

func moduleReady() bool {
	out, err := exec.Command("jool", "instance", "display").CombinedOutput()
	if err == nil {
		return true
	}
	return !strings.Contains(string(out), "hasn't been modprobed")
}

// Apply 配置 Jool NAT64 实例（关闭时移除）
func Apply(nat store.NatState) error {
	_ = removeInstance()
	if !nat.Nat64Enabled {
		return nil
	}
	if !Installed() {
		return fmt.Errorf("jool not installed (apt install jool-tools jool-dkms)")
	}
	if !moduleReady() {
		return fmt.Errorf("jool kernel module not loaded (modprobe jool)")
	}
	prefix := strings.TrimSpace(nat.Nat64Prefix)
	if prefix == "" {
		prefix = store.DefaultNat64Prefix
	}
	pool4 := strings.TrimSpace(nat.Nat64Pool4)
	if pool4 == "" {
		pool4 = store.DefaultNat64Pool4
	}
	if err := run("jool", "instance", "add", instanceName, "--netfilter", "--pool6", prefix); err != nil {
		return fmt.Errorf("jool instance add: %w", err)
	}
	for _, proto := range []string{"--udp", "--tcp"} {
		if err := run("jool", "-i", instanceName, "pool4", "add", pool4, "1024-65535", proto); err != nil {
			_ = removeInstance()
			return fmt.Errorf("jool pool4 add: %w", err)
		}
	}
	return nil
}

func removeInstance() error {
	if !Installed() {
		return nil
	}
	_ = run("jool", "instance", "remove", instanceName)
	return nil
}

// Active 检查实例是否存在
func Active() bool {
	if !Installed() || !moduleReady() {
		return false
	}
	out, err := exec.Command("jool", "instance", "display").CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(string(out), instanceName)
}

func run(name string, args ...string) error {
	out, err := exec.Command(name, args...).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("%s", msg)
	}
	return nil
}
