package ebpf

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cilium/ebpf"
)

const ifbDev = "ifb0"

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

// AttachTC：仅重装 LAN egress 下行 BPF；勿 flush ifb0 parent 1:（会删掉上行 per-IP u32）
func (m *Manager) AttachTC(devLAN string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return fmt.Errorf("ebpf not loaded")
	}
	egressPin := filepath.Join(PinDir, progEgress)
	if err := m.attachBPFFilterHTB(devLAN, egressPin); err != nil {
		return err
	}
	if m.attached == nil {
		m.attached = map[string]struct{}{}
	}
	m.attached[devLAN] = struct{}{}
	m.attached[ifbDev] = struct{}{}
	m.attachedDev = devLAN
	return nil
}

// AttachLANEgressBPF 在 LAN HTB 上挂下行 classify_egress（replay 动态类后补挂）
func (m *Manager) AttachLANEgressBPF(devLAN string) error {
	if devLAN == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return fmt.Errorf("ebpf not loaded")
	}
	egressPin := filepath.Join(PinDir, progEgress)
	return m.attachBPFFilterHTB(devLAN, egressPin)
}

// AttachTCDevice 附加接口（如 wg0）ingress+egress
func (m *Manager) AttachTCDevice(dev string) error {
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
	if err := m.attachBPFFilterLocked(dev, "ingress", ingressPin); err != nil {
		return err
	}
	if err := m.attachBPFFilterLocked(dev, "egress", egressPin); err != nil {
		return err
	}
	if m.attached == nil {
		m.attached = map[string]struct{}{}
	}
	m.attached[dev] = struct{}{}
	return nil
}

// AttachTCDeviceEgressOnly 在指定设备上挂 classify_egress，与 LAN 一致使用 HTB parent 1:（勿用 clsact egress：
// clsact 上 direct-action 设置的 tc_classid 无法稳定参与 root HTB 选类，WG 下行会落默认类导致限速失效）。
// ingress 仍留给 u32+mirred；会先清空 ingress 上遗留 filter（由上层随后重装 mirred）。
func (m *Manager) AttachTCDeviceEgressOnly(dev string) error {
	if dev == "" {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.loaded || m.objs == nil {
		return fmt.Errorf("ebpf not loaded")
	}
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
	egressPin := filepath.Join(PinDir, progEgress)
	if err := m.attachBPFFilterHTB(dev, egressPin); err != nil {
		return err
	}
	if m.attached == nil {
		m.attached = map[string]struct{}{}
	}
	m.attached[dev] = struct{}{}
	return nil
}

// flushHTBBPFOnly 仅移除 HTB 上的 BPF 分类器，保留 u32/flower 等整形规则
func flushHTBBPFOnly(dev string) {
	const bpfPrio = "49152"
	for i := 0; i < 8; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", dev, "parent", "1:",
			"protocol", "all", "prio", bpfPrio).CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") {
			break
		}
	}
}

func (m *Manager) attachBPFFilterHTB(dev, pin string) error {
	flushHTBBPFOnly(dev)
	out, err := exec.Command("tc", "filter", "add", "dev", dev, "parent", "1:",
		"protocol", "all", "prio", "49152", "bpf",
		"direct-action", "object-pinned", pin, "classid", "1:0").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			log.Printf("tc bpf %s parent 1: pin=%s: %s", dev, pin, msg)
			return fmt.Errorf("tc filter %s parent 1: %s %w", dev, msg, err)
		}
	}
	return nil
}

func (m *Manager) attachBPFFilterLocked(dev, dir, pin string) error {
	_ = exec.Command("tc", "filter", "del", "dev", dev, dir).Run()
	out, err := exec.Command("tc", "filter", "add", "dev", dev, dir, "bpf",
		"direct-action", "object-pinned", pin, "classid", "1:0").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
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
	flushHTBBPFOnly(dev)
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
		_ = exec.Command("tc", "filter", "del", "dev", dev, "parent", "1:", "protocol", "all").Run()
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
