package ebpf

import (
	"os/exec"
	"path/filepath"
	"strings"
)

// ProgramStatus TC/BPF 程序挂载状态
type ProgramStatus struct {
	Name     string `json:"name"`
	PinPath  string `json:"pin_path,omitempty"`
	Attached string `json:"attached,omitempty"`
	Kind     string `json:"kind"`
}

// ListPrograms 返回 pinned 程序与 tc filter 摘要
func (m *Manager) ListPrograms(devLAN string) []ProgramStatus {
	var out []ProgramStatus
	for _, name := range []string{progIngress, progEgress} {
		pin := filepath.Join(PinDir, name)
		st := ProgramStatus{Name: name, PinPath: pin, Kind: "tc_bpf"}
		if devLAN != "" {
			dir := "ingress"
			if name == progEgress {
				dir = "egress"
			}
			if line := tcFilterLine(devLAN, dir); line != "" {
				st.Attached = devLAN + " " + dir + ": " + line
			}
		}
		out = append(out, st)
	}
	return out
}

func tcFilterLine(dev, dir string) string {
	out, err := exec.Command("tc", "filter", "show", "dev", dev, dir).CombinedOutput()
	if err != nil {
		return ""
	}
	lines := strings.Split(string(out), "\n")
	for _, l := range lines {
		if strings.Contains(l, "bpf") || strings.Contains(l, "classify") {
			return strings.TrimSpace(l)
		}
	}
	if len(lines) > 1 {
		return strings.TrimSpace(lines[1])
	}
	return ""
}

// Reload 维护窗口：Detach → Close → Load → Attach（调用方负责 ReplayState）
func (m *Manager) Reload(devLAN string) error {
	_ = m.DetachTC()
	_ = m.Close()
	if err := m.Load(); err != nil {
		return err
	}
	return m.AttachTC(devLAN)
}
