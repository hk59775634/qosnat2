package ebpf

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cilium/ebpf"
)

const (
	progIngress = "classify_ingress"
	progEgress  = "classify_egress"
)

func ifbIndex() (int, error) {
	out, err := exec.Command("ip", "-j", "link", "show", "ifb0").CombinedOutput()
	if err != nil {
		out2, err2 := exec.Command("ip", "link", "show", "ifb0").CombinedOutput()
		if err2 != nil {
			return 0, fmt.Errorf("ifb0: %w", err)
		}
		fields := strings.Fields(string(out2))
		for i, f := range fields {
			if f == "ifb0" && i > 0 {
				var idx int
				if _, err := fmt.Sscanf(fields[0], "%d:", &idx); err == nil {
					return idx, nil
				}
			}
		}
		return 0, fmt.Errorf("parse ifb0 index")
	}
	s := string(out)
	i := strings.Index(s, `"ifindex":`)
	if i < 0 {
		return 0, fmt.Errorf("ifindex not in json")
	}
	var idx int
	_, err = fmt.Sscanf(s[i:], `"ifindex":%d`, &idx)
	return idx, err
}

func (m *Manager) rewriteIFBIndex(spec *ebpf.CollectionSpec) error {
	idx, err := ifbIndex()
	if err != nil {
		return err
	}
	return spec.RewriteConstants(map[string]interface{}{
		"ifb_ifindex": int32(idx),
	})
}

func (m *Manager) pinPrograms(objs *bpfObjects) error {
	progs := []struct {
		name string
		prog *ebpf.Program
	}{
		{progIngress, objs.Ingress},
		{progEgress, objs.Egress},
	}
	for _, p := range progs {
		path := filepath.Join(PinDir, p.name)
		_ = os.Remove(path)
		if err := p.prog.Pin(path); err != nil {
			return fmt.Errorf("pin %s: %w", p.name, err)
		}
	}
	return nil
}

// AttachTC 将 pinned 程序挂到 LAN clsact（direct-action）
func (m *Manager) AttachTC(devLAN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return fmt.Errorf("ebpf not loaded")
	}
	return m.attachDeviceLocked(devLAN)
}

// AttachTCDevice 附加接口（如 wg0）复用同一套 classify 程序
func (m *Manager) AttachTCDevice(dev string) error {
	if dev == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return fmt.Errorf("ebpf not loaded")
	}
	return m.attachDeviceLocked(dev)
}

func (m *Manager) attachDeviceLocked(dev string) error {
	if _, ok := m.attached[dev]; ok {
		return nil
	}
	_ = exec.Command("tc", "filter", "del", "dev", dev, "ingress").Run()
	_ = exec.Command("tc", "filter", "del", "dev", dev, "egress").Run()

	ingressPin := filepath.Join(PinDir, progIngress)
	egressPin := filepath.Join(PinDir, progEgress)
	for _, spec := range []struct {
		dir, pin string
	}{
		{"ingress", ingressPin},
		{"egress", egressPin},
	} {
		out, err := exec.Command("tc", "filter", "add", "dev", dev, spec.dir, "bpf",
			"direct-action", "object-pinned", spec.pin).CombinedOutput()
		if err != nil {
			return fmt.Errorf("tc filter %s %s: %s %w", dev, spec.dir, strings.TrimSpace(string(out)), err)
		}
	}
	if m.attached == nil {
		m.attached = map[string]struct{}{}
	}
	m.attached[dev] = struct{}{}
	if m.attachedDev == "" {
		m.attachedDev = dev
	}
	return nil
}

func (m *Manager) DetachTC() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.detachLocked()
}

// DetachTCDevice 仅卸载指定接口上的 BPF 分类
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

func (m *Manager) AttachedDev() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.attachedDev
}
