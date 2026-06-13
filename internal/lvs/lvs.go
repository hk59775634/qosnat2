package lvs

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

// Config 运行时参数。
type Config struct {
	DevWAN string
	State  store.LVSState
}

// Status ipvs 安装与规则摘要。
type Status struct {
	Installed bool   `json:"installed"`
	Active    bool   `json:"active"`
	Summary   string `json:"summary,omitempty"`
}

func installed() bool {
	_, err := exec.LookPath("ipvsadm")
	return err == nil
}

// ShowStatus 返回 ipvsadm 是否可用及当前规则行数。
func ShowStatus() Status {
	st := Status{Installed: installed()}
	if !st.Installed {
		return st
	}
	out, err := exec.Command("ipvsadm", "-Ln").CombinedOutput()
	if err != nil {
		return st
	}
	lines := 0
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "TCP") || strings.HasPrefix(line, "UDP") {
			lines++
		}
	}
	st.Active = lines > 0
	if lines > 0 {
		st.Summary = fmt.Sprintf("%d virtual service(s)", lines)
	}
	return st
}

// Apply 写入 IPVS 规则；disabled 时清空。
func Apply(cfg Config) error {
	if !installed() {
		return fmt.Errorf("ipvsadm not installed (apt install ipvsadm)")
	}
	st := cfg.State
	_ = store.NormalizeLVS(&st, cfg.DevWAN)
	if err := ensureModules(st.Mode); err != nil {
		return err
	}
	_ = exec.Command("sysctl", "-w", "net.ipv4.vs.conntrack=1").Run()
	if !st.Enabled {
		return clearRules()
	}
	for _, vs := range st.VirtualServers {
		if vs.AutoVIP && vs.WANDevice != "" {
			if err := ensureVIP(vs.WANDevice, vs.VIP); err != nil {
				return err
			}
		}
	}
	if err := clearRules(); err != nil {
		return err
	}
	for _, vs := range st.VirtualServers {
		for _, proto := range store.LVSProtos(vs.Protocol) {
			if err := addVirtualServerProto(vs, proto, st.Mode); err != nil {
				return err
			}
		}
	}
	return nil
}

func ensureModules(mode string) error {
	mods := []string{"ip_vs"}
	switch mode {
	case "nat":
		mods = append(mods, "ip_vs_nat")
	case "dr":
		mods = append(mods, "ip_vs_rr")
	}
	for _, m := range mods {
		if out, err := exec.Command("modprobe", m).CombinedOutput(); err != nil {
			msg := strings.TrimSpace(string(out))
			return fmt.Errorf("modprobe %s: %s %w", m, msg, err)
		}
	}
	return nil
}

func ensureVIP(dev, vip string) error {
	if !route.LinkExists(dev) {
		return fmt.Errorf("interface %s not found", dev)
	}
	out, _ := exec.Command("ip", "-4", "addr", "show", "dev", dev).CombinedOutput()
	if strings.Contains(string(out), vip+"/") || strings.Contains(string(out), " "+vip+" ") {
		return nil
	}
	cidr := vip + "/32"
	if out, err := exec.Command("ip", "addr", "add", cidr, "dev", dev).CombinedOutput(); err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "File exists") {
			return nil
		}
		return fmt.Errorf("ip addr add %s dev %s: %s %w", cidr, dev, msg, err)
	}
	return nil
}

func clearRules() error {
	out, err := exec.Command("ipvsadm", "-C").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg != "" && !strings.Contains(msg, "No chain") {
			return fmt.Errorf("ipvsadm -C: %s %w", msg, err)
		}
	}
	return nil
}

func ipvsProtoFlag(proto string) string {
	switch strings.ToLower(strings.TrimSpace(proto)) {
	case "udp":
		return "u"
	default:
		return "t"
	}
}

func addVirtualServerProto(vs store.LVSVirtualServer, proto, mode string) error {
	fwd := forwardFlag(mode)
	service := fmt.Sprintf("%s:%d", vs.VIP, vs.Port)
	flag := ipvsProtoFlag(proto)
	args := []string{"-A", "-" + flag, service, "-s", vs.Scheduler}
	if vs.PersistenceSec > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", vs.PersistenceSec))
	}
	if out, err := exec.Command("ipvsadm", args...).CombinedOutput(); err != nil {
		return fmt.Errorf("ipvsadm add vs %s %s: %s %w", proto, service, strings.TrimSpace(string(out)), err)
	}
	for _, rs := range vs.RealServers {
		backend := fmt.Sprintf("%s:%d", rs.IP, rs.Port)
		rargs := []string{"-a", "-" + flag, service, "-r", backend, fwd, "-w", fmt.Sprintf("%d", rs.Weight)}
		if out, err := exec.Command("ipvsadm", rargs...).CombinedOutput(); err != nil {
			return fmt.Errorf("ipvsadm add rs %s %s: %s %w", proto, backend, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func forwardFlag(mode string) string {
	if mode == "dr" {
		return "-g"
	}
	return "-m"
}

// Install apt 安装 ipvsadm。
func Install() error {
	if installed() {
		return nil
	}
	out, err := exec.Command("apt-get", "install", "-y", "-qq", "ipvsadm").CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt install ipvsadm: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
