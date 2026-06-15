package lvs

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

const modulesLoadPath = "/etc/modules-load.d/qosnat2-ipvs.conf"

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

// Apply 写入 IPVS 规则（Director）或 RS 本机配置；disabled 时清空。
func Apply(cfg Config) error {
	st := cfg.State
	if err := store.NormalizeLVS(&st, cfg.DevWAN); err != nil {
		return err
	}
	role := store.LVSRole(&st)
	if !st.Enabled {
		_ = ClearRS(st)
		if role == store.LVSRoleRS {
			return nil
		}
		return clearRules()
	}
	if role == store.LVSRoleRS {
		return ApplyRS(st)
	}
	_ = ClearRS(st)
	if !installed() {
		return fmt.Errorf("ipvsadm not installed (apt install ipvsadm)")
	}
	if err := ensureModules(st.Mode); err != nil {
		return err
	}
	_ = exec.Command("sysctl", "-w", "net.ipv4.vs.conntrack=1").Run()
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
	if err := ensureIPVSModule(); err != nil {
		return err
	}
	// 6.x 内核无独立 ip_vs_nat；NAT (-m) 由 ip_vs + nf_nat 完成。
	if mode == "dr" {
		_ = modprobeModule("ip_vs_rr")
	}
	return nil
}

func moduleLoaded(name string) bool {
	data, err := os.ReadFile("/proc/modules")
	if err != nil {
		return false
	}
	prefix := name + " "
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, prefix) {
			return true
		}
	}
	return false
}

func modprobeModule(name string) error {
	if moduleLoaded(name) {
		return nil
	}
	out, err := exec.Command("modprobe", name).CombinedOutput()
	if err == nil || moduleLoaded(name) {
		return nil
	}
	msg := strings.TrimSpace(string(out))
	return fmt.Errorf("modprobe %s: %s %w", name, msg, err)
}

func ensureIPVSModule() error {
	if moduleLoaded("ip_vs") {
		return nil
	}
	if err := modprobeModule("ip_vs"); err == nil {
		return nil
	}
	if err := EnsureKernelModules(); err != nil {
		return err
	}
	if err := modprobeModule("ip_vs"); err != nil {
		kver := runningKernel()
		return fmt.Errorf("%w; if kernel was upgraded recently, install linux-modules-extra-%s and reboot", err, kver)
	}
	return nil
}

func runningKernel() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// kernelModuleFileExists 检查当前运行内核的模块树中是否存在 .ko/.ko.zst（比 modinfo 更可靠）。
func kernelModuleFileExists(name string) bool {
	kver := runningKernel()
	if kver == "" {
		return false
	}
	base := filepath.Join("/lib/modules", kver)
	for _, rel := range []string{
		filepath.Join("kernel", "net", "netfilter", "ipvs", name+".ko"),
		filepath.Join("kernel", "net", "netfilter", "ipvs", name+".ko.zst"),
	} {
		if _, err := os.Stat(filepath.Join(base, rel)); err == nil {
			return true
		}
	}
	return false
}

// EnsureKernelModules 写入 modules-load.d，并在缺少 ip_vs 时安装 linux-modules-extra。
func EnsureKernelModules() error {
	if err := os.WriteFile(modulesLoadPath, []byte("ip_vs\n"), 0644); err != nil && !os.IsExist(err) {
		return fmt.Errorf("write %s: %w", modulesLoadPath, err)
	}
	if moduleLoaded("ip_vs") || kernelModuleFileExists("ip_vs") {
		return nil
	}
	kver := runningKernel()
	if kver == "" {
		return fmt.Errorf("uname -r failed")
	}
	pkg := "linux-modules-extra-" + kver
	_ = exec.Command("apt-get", "update", "-qq").Run()
	out, err := exec.Command("apt-get", "install", "-y", "-qq", pkg).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		return fmt.Errorf("kernel module ip_vs missing for %s: run apt install %s: %s %w", kver, pkg, msg, err)
	}
	_ = exec.Command("depmod", "-a", kver).Run()
	if !kernelModuleFileExists("ip_vs") {
		return fmt.Errorf("ip_vs still missing after installing %s; reboot if the running kernel was recently upgraded", pkg)
	}
	return nil
}

func moduleAvailable(name string) bool {
	if moduleLoaded(name) {
		return true
	}
	return kernelModuleFileExists(name)
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
	if p := store.LVSPersistenceSec(vs, proto); p > 0 {
		args = append(args, "-p", fmt.Sprintf("%d", p))
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

// Install apt 安装 ipvsadm 与 IPVS 内核模块依赖。
func Install() error {
	if err := EnsureKernelModules(); err != nil {
		return err
	}
	if err := ensureIPVSModule(); err != nil {
		return err
	}
	if installed() {
		return nil
	}
	out, err := exec.Command("apt-get", "install", "-y", "-qq", "ipvsadm").CombinedOutput()
	if err != nil {
		return fmt.Errorf("apt install ipvsadm: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}
