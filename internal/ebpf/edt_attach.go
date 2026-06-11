package ebpf

import (
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// AttachTCEDT 在 dev 的 clsact ingress/egress 挂载 rate_limit_* BPF
func (m *Manager) AttachTCEDT(dev string) error {
	if dev == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.edtObjs == nil {
		return fmt.Errorf("edt ebpf not loaded")
	}
	ingressPin := filepath.Join(PinDir, edtProgIngress)
	egressPin := filepath.Join(PinDir, edtProgEgress)
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
	if err := m.attachEDTFilter(dev, "ingress", ingressPin); err != nil {
		return err
	}
	if err := m.attachEDTFilter(dev, "egress", egressPin); err != nil {
		return err
	}
	if m.attached == nil {
		m.attached = map[string]struct{}{}
	}
	m.attached[dev] = struct{}{}
	m.attachedDev = dev
	return nil
}

// AttachTCDeviceEDT WireGuard / ocserv 等：ingress + egress EDT
func (m *Manager) AttachTCDeviceEDT(dev string) error {
	return m.AttachTCEDT(dev)
}

func (m *Manager) attachEDTFilter(dev, dir, pin string) error {
	out, err := exec.Command("tc", "filter", "add", "dev", dev, dir, "bpf",
		"direct-action", "object-pinned", pin).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			log.Printf("tc edt bpf %s %s pin=%s: %s", dev, dir, pin, msg)
			return fmt.Errorf("tc edt filter %s %s: %s %w", dev, dir, msg, err)
		}
	}
	return nil
}
