package ebpf

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// AttachTC 在 dev 的 clsact ingress/egress 挂载 rate_limit_* BPF。
func (m *Manager) AttachTC(dev string) error {
	if dev == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return fmt.Errorf("ebpf not loaded")
	}
	ingressPin := filepath.Join(PinDir, progIngress)
	egressPin := filepath.Join(PinDir, progEgress)
	for i := 0; i < 32; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", dev, "ingress").CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") ||
			strings.Contains(msg, "does not match") {
			break
		}
	}
	for i := 0; i < 32; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", dev, "egress").CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") ||
			strings.Contains(msg, "does not match") {
			break
		}
	}
	if err := m.attachFilter(dev, "ingress", ingressPin); err != nil {
		return err
	}
	if err := m.attachFilter(dev, "egress", egressPin); err != nil {
		return err
	}
	if m.attached == nil {
		m.attached = map[string]struct{}{}
	}
	m.attached[dev] = struct{}{}
	m.attachedDev = dev
	return nil
}

// AttachTCDevice WireGuard / ocserv 等：ingress + egress EDT。
func (m *Manager) AttachTCDevice(dev string) error {
	return m.AttachTC(dev)
}

func (m *Manager) attachFilter(dev, dir, pin string) error {
	out, err := exec.Command("tc", "filter", "add", "dev", dev, dir, "bpf",
		"direct-action", "object-pinned", pin).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			log.Printf("tc bpf %s %s pin=%s: %s", dev, dir, pin, msg)
			return fmt.Errorf("tc filter %s %s: %s %w", dev, dir, msg, err)
		}
	}
	return nil
}

func (m *Manager) DetachTC() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.detachLocked()
}

// DetachTCDevice 卸载指定接口上的 BPF 分类。
func (m *Manager) DetachTCDevice(dev string) error {
	if dev == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.attached[dev]; !ok {
		return nil
	}
	delete(m.attached, dev)
	_ = exec.Command("tc", "filter", "del", "dev", dev, "ingress").Run()
	_ = exec.Command("tc", "filter", "del", "dev", dev, "egress").Run()
	if m.attachedDev == dev {
		m.attachedDev = ""
		for d := range m.attached {
			m.attachedDev = d
			break
		}
	}
	return nil
}

func (m *Manager) detachLocked() error {
	for dev := range m.attached {
		_ = exec.Command("tc", "filter", "del", "dev", dev, "ingress").Run()
		_ = exec.Command("tc", "filter", "del", "dev", dev, "egress").Run()
	}
	m.attached = map[string]struct{}{}
	m.attachedDev = ""
	return nil
}

func (m *Manager) AttachedDev() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.attachedDev
}
