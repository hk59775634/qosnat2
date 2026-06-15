package lvs

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

const rsSysctlPath = "/etc/sysctl.d/99-qosnat2-lvs-rs.conf"

// RSStatus DR Real Server 运行时摘要。
type RSStatus struct {
	Active      bool     `json:"active"`
	LoVIPs      []string `json:"lo_vips,omitempty"`
	ArpIgnore   int      `json:"arp_ignore,omitempty"`
	ArpAnnounce int      `json:"arp_announce,omitempty"`
	Summary     string   `json:"summary,omitempty"`
}

// ShowRSStatus 检查 lo VIP 与 ARP 参数。
func ShowRSStatus(st store.LVSState) RSStatus {
	out := RSStatus{}
	if store.LVSRole(&st) != store.LVSRoleRS {
		return out
	}
	out.LoVIPs = loIPv4Addrs()
	out.ArpIgnore = sysctlInt("net.ipv4.conf.all.arp_ignore")
	out.ArpAnnounce = sysctlInt("net.ipv4.conf.all.arp_announce")
	if !st.Enabled {
		return out
	}
	want := map[string]struct{}{}
	for _, e := range st.RS.Entries {
		want[e.VIP] = struct{}{}
	}
	matched := 0
	for vip := range want {
		if loHasVIP(vip) {
			matched++
		}
	}
	out.Active = matched > 0 && out.ArpIgnore == 1 && out.ArpAnnounce == 2
	if matched > 0 {
		out.Summary = fmt.Sprintf("%d VIP on lo", matched)
	}
	return out
}

// ApplyRS 配置 DR Real Server：lo 绑 VIP、ARP 抑制；不运行 ipvsadm。
func ApplyRS(st store.LVSState) error {
	_ = store.NormalizeLVS(&st, "")
	if err := clearRules(); err != nil {
		return err
	}
	if !st.Enabled {
		return ClearRS(st)
	}
	want := map[string]struct{}{}
	for _, e := range st.RS.Entries {
		want[e.VIP] = struct{}{}
	}
	for vip := range want {
		if err := ensureLoVIP(vip); err != nil {
			return err
		}
	}
	for _, vip := range loIPv4Addrs() {
		if _, keep := want[vip]; keep {
			continue
		}
		if err := removeLoVIP(vip); err != nil {
			return err
		}
	}
	return applyRSSysctl(true)
}

// ClearRS 移除本机 RS 配置（lo VIP 与专用 sysctl）。
func ClearRS(st store.LVSState) error {
	for _, e := range st.RS.Entries {
		_ = removeLoVIP(e.VIP)
	}
	return applyRSSysctl(false)
}

func ensureLoVIP(vip string) error {
	if loHasVIP(vip) {
		return nil
	}
	cidr := vip + "/32"
	out, err := exec.Command("ip", "addr", "add", cidr, "dev", "lo").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "File exists") {
			return nil
		}
		return fmt.Errorf("ip addr add %s dev lo: %s %w", cidr, msg, err)
	}
	return nil
}

func removeLoVIP(vip string) error {
	if !loHasVIP(vip) {
		return nil
	}
	cidr := vip + "/32"
	out, err := exec.Command("ip", "addr", "del", cidr, "dev", "lo").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "Cannot assign") || strings.Contains(msg, "not found") {
			return nil
		}
		return fmt.Errorf("ip addr del %s dev lo: %s %w", cidr, msg, err)
	}
	return nil
}

func loHasVIP(vip string) bool {
	out, _ := exec.Command("ip", "-4", "addr", "show", "dev", "lo").CombinedOutput()
	s := string(out)
	return strings.Contains(s, vip+"/") || strings.Contains(s, " "+vip+" ")
}

func loIPv4Addrs() []string {
	out, err := exec.Command("ip", "-4", "-o", "addr", "show", "dev", "lo").CombinedOutput()
	if err != nil {
		return nil
	}
	var ips []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		addr := strings.Split(fields[3], "/")[0]
		if addr == "" || addr == "127.0.0.1" {
			continue
		}
		ips = append(ips, addr)
	}
	return ips
}

func applyRSSysctl(enable bool) error {
	if !enable {
		_ = os.Remove(rsSysctlPath)
		_ = exec.Command("sysctl", "-w", "net.ipv4.conf.all.arp_ignore=0").Run()
		_ = exec.Command("sysctl", "-w", "net.ipv4.conf.all.arp_announce=0").Run()
		_ = exec.Command("sysctl", "-w", "net.ipv4.conf.lo.arp_ignore=0").Run()
		_ = exec.Command("sysctl", "-w", "net.ipv4.conf.lo.arp_announce=0").Run()
		return nil
	}
	body := strings.Join([]string{
		"net.ipv4.conf.all.arp_ignore = 1",
		"net.ipv4.conf.all.arp_announce = 2",
		"net.ipv4.conf.lo.arp_ignore = 1",
		"net.ipv4.conf.lo.arp_announce = 2",
		"",
	}, "\n")
	if err := os.WriteFile(rsSysctlPath, []byte(body), 0644); err != nil {
		return fmt.Errorf("write %s: %w", rsSysctlPath, err)
	}
	out, err := exec.Command("sysctl", "-p", rsSysctlPath).CombinedOutput()
	if err != nil {
		return fmt.Errorf("sysctl -p %s: %s %w", rsSysctlPath, strings.TrimSpace(string(out)), err)
	}
	return nil
}

func sysctlInt(key string) int {
	out, err := exec.Command("sysctl", "-n", key).Output()
	if err != nil {
		return -1
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}
