// Package warpnetns 在独立网络命名空间中运行 Cloudflare WARP，避免劫持宿主机默认路由。
// 思路参考 vopono：warp-svc / warp-cli 仅在 netns 内连接，隧道网卡再移到宿主机供策略路由使用。
package warpnetns

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	NetnsName    = "qosnat2-warp"
	VethHost     = "qwp0"
	VethNS       = "qwp1"
	HostVethCIDR = "10.99.0.1/30"
	NSVethCIDR   = "10.99.0.2/30"
	NSVethGW     = "10.99.0.1"

	stateFile = "/var/lib/qosnat2/warp-netns.json"
	warpSvc   = "/usr/bin/warp-svc"
	warpCLI   = "/usr/bin/warp-cli"
)

type State struct {
	Netns     string `json:"netns"`
	SvcPID    int    `json:"svc_pid,omitempty"`
	HostIface string `json:"host_iface,omitempty"`
	UplinkDev string `json:"uplink_dev,omitempty"`
	Connected bool   `json:"connected"`
}

func loadState() State {
	st := State{Netns: NetnsName}
	b, err := os.ReadFile(stateFile)
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if st.Netns == "" {
		st.Netns = NetnsName
	}
	return st
}

func saveState(st State) {
	_ = os.MkdirAll(filepath.Dir(stateFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(stateFile, b, 0600)
}

func netnsExists() bool {
	_, err := os.Stat(filepath.Join("/var/run/netns", NetnsName))
	return err == nil
}

// netnsUsable netns 存在且 ip netns exec 可用（排除 Peer netns reference is invalid 等损坏状态）。
func netnsUsable() bool {
	if !netnsExists() {
		return false
	}
	_, err := netnsExec("true")
	return err == nil
}

// vethPairHealthy 宿主机 qwp0 与 netns 内 qwp1 成对且可用。
func vethPairHealthy() bool {
	if !linkExists(VethHost) {
		return false
	}
	if !netnsUsable() {
		return false
	}
	_, err := netnsExec("ip", "link", "show", VethNS)
	return err == nil
}

// needsNetnsReset 检测孤儿 veth 或 netns 损坏（常见于 WARP 连接中断后）。
func needsNetnsReset() bool {
	if linkExists(VethHost) && !vethPairHealthy() {
		return true
	}
	if netnsExists() && !netnsUsable() {
		return true
	}
	return false
}

// forceResetNetns 强制拆除损坏的 netns/veth，便于 Ensure 干净重建。
func forceResetNetns() {
	st := loadState()
	uplink := strings.TrimSpace(st.UplinkDev)
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if netnsUsable() {
		if warpDaemonReady() {
			_, _ = netnsExec(warpCLI, "--accept-tos", "disconnect")
		}
		stopSvcInNetns()
		deleteWarpLinksInNetns()
		deleteLinkInNetns(VethNS)
	}
	hostIface := strings.TrimSpace(st.HostIface)
	if hostIface == "" {
		hostIface = detectHostWarpIface()
	}
	deleteLink(hostIface)
	deleteLink(VethHost)
	removeNATRule(uplink)
	if netnsExists() {
		_, _ = run("ip", "netns", "delete", NetnsName)
	}
	saveState(State{Netns: NetnsName})
}

func run(args ...string) ([]byte, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return out, err
}

func netnsExec(args ...string) ([]byte, error) {
	full := append([]string{"netns", "exec", NetnsName}, args...)
	return run(append([]string{"ip"}, full...)...)
}

func nftEnsureHostWarpUplinkMasq(uplink string) {
	if strings.TrimSpace(uplink) == "" {
		return
	}
	_, _ = run("nft", "add", "table", "ip", "qosnat2_warp")
	_, _ = run("nft", "add", "chain", "ip", "qosnat2_warp", "postrouting",
		"{", "type", "nat", "hook", "postrouting", "priority", "srcnat", ";", "policy", "accept", ";", "}")
	rule := fmt.Sprintf(`ip saddr 10.99.0.0/30 oifname "%s" masquerade`, uplink)
	_, _ = run("nft", "add", "rule", "ip", "qosnat2_warp", "postrouting", "ip", "saddr", "10.99.0.0/30", "oifname", uplink, "masquerade")
	// 去重：保留一条同内容规则即可
	out, err := run("nft", "-a", "list", "chain", "ip", "qosnat2_warp", "postrouting")
	if err != nil {
		return
	}
	seen := 0
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, rule) || !strings.Contains(line, "handle ") {
			continue
		}
		seen++
		if seen <= 1 {
			continue
		}
		fields := strings.Fields(line)
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "handle" {
				_, _ = run("nft", "delete", "rule", "ip", "qosnat2_warp", "postrouting", "handle", fields[i+1])
				break
			}
		}
	}
}

func nftRemoveHostWarpUplinkMasq(uplink string) {
	if strings.TrimSpace(uplink) == "" {
		return
	}
	rule := fmt.Sprintf(`ip saddr 10.99.0.0/30 oifname "%s" masquerade`, uplink)
	out, err := run("nft", "-a", "list", "chain", "ip", "qosnat2_warp", "postrouting")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(line, rule) || !strings.Contains(line, "handle ") {
			continue
		}
		fields := strings.Fields(line)
		for i := 0; i < len(fields)-1; i++ {
			if fields[i] == "handle" {
				_, _ = run("nft", "delete", "rule", "ip", "qosnat2_warp", "postrouting", "handle", fields[i+1])
				break
			}
		}
	}
}

func nftEnsureNetnsWarpMasq(warpIface string) {
	if strings.TrimSpace(warpIface) == "" || !netnsExists() {
		return
	}
	_, _ = netnsExec("nft", "add", "table", "ip", "qosnat2_warp")
	_, _ = netnsExec("nft", "add", "chain", "ip", "qosnat2_warp", "postrouting",
		"{", "type", "nat", "hook", "postrouting", "priority", "srcnat", ";", "policy", "accept", ";", "}")
	_, _ = netnsExec("nft", "add", "rule", "ip", "qosnat2_warp", "postrouting", "oifname", warpIface, "masquerade")
}

func cleanupLegacyIPTablesNAT(uplink, warpIface string) {
	if strings.TrimSpace(uplink) != "" {
		_ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", "10.99.0.0/30", "-o", uplink, "-j", "MASQUERADE").Run()
	}
	if strings.TrimSpace(warpIface) != "" && netnsExists() {
		_ = exec.Command("ip", "netns", "exec", NetnsName, "iptables", "-t", "nat", "-D", "POSTROUTING",
			"-o", warpIface, "-j", "MASQUERADE").Run()
		_ = exec.Command("ip", "netns", "exec", NetnsName, "iptables", "-t", "nat", "-D", "POSTROUTING",
			"-s", "10.99.0.0/30", "-o", warpIface, "-j", "MASQUERADE").Run()
	}
}

func pidIsUsable(pid int) bool {
	if pid <= 0 {
		return false
	}
	b, err := os.ReadFile(filepath.Join("/proc", strconv.Itoa(pid), "stat"))
	if err != nil {
		return false
	}
	// /proc/<pid>/stat format: pid (comm) state ...
	s := string(b)
	r := strings.LastIndex(s, ") ")
	if r < 0 || r+2 >= len(s) {
		return false
	}
	state := s[r+2]
	return state != 'Z'
}

func warpDaemonReady() bool {
	out, err := netnsExec(warpCLI, "--accept-tos", "status")
	if err != nil {
		return false
	}
	low := strings.ToLower(string(out))
	return !strings.Contains(low, "unable to connect")
}

// StopHostWarpSvc 停止宿主机上的 warp-svc，避免与 netns 实例冲突。
func StopHostWarpSvc() {
	_, _ = exec.Command("systemctl", "stop", "warp-svc").CombinedOutput()
	// 若 systemd 未托管，尝试结束宿主机命名空间中的 warp-svc
	out, _ := exec.Command("pgrep", "-x", "warp-svc").Output()
	for _, pid := range strings.Fields(string(out)) {
		ns, _ := os.Readlink(filepath.Join("/proc", pid, "ns/net"))
		self, _ := os.Readlink("/proc/self/ns/net")
		if ns == self {
			_, _ = exec.Command("kill", pid).CombinedOutput()
		}
	}
	time.Sleep(500 * time.Millisecond)
}

func mainUplinkDev() string {
	out, err := exec.Command("ip", "-4", "route", "show", "default").Output()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		for i, f := range fields {
			if f == "dev" && i+1 < len(fields) {
				return fields[i+1]
			}
		}
	}
	return ""
}

// Ensure 创建 netns、veth，并为 netns 提供经主 WAN 的 NAT 出口（供 WARP 建连）。
func Ensure(uplink string) error {
	if needsNetnsReset() {
		forceResetNetns()
	}
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if uplink == "" {
		return fmt.Errorf("cannot detect main WAN device for warp netns uplink")
	}
	if !netnsExists() {
		if out, err := run("ip", "netns", "add", NetnsName); err != nil {
			return fmt.Errorf("ip netns add: %s %w", strings.TrimSpace(string(out)), err)
		}
	}
	_, _ = netnsExec("ip", "link", "set", "lo", "up")
	// veth：若宿主机残留 qwp0 但 netns 内无 qwp1，先拆除再建。
	if linkExists(VethHost) {
		if _, err := netnsExec("ip", "link", "show", VethNS); err != nil {
			deleteLink(VethHost)
		}
	}
	if !linkExists(VethHost) {
		if out, err := run("ip", "link", "add", VethHost, "type", "veth", "peer", "name", VethNS); err != nil {
			return fmt.Errorf("veth: %s %w", strings.TrimSpace(string(out)), err)
		}
	}
	if _, err := netnsExec("ip", "link", "show", VethNS); err != nil {
		if out, err := run("ip", "link", "set", VethNS, "netns", NetnsName); err != nil {
			forceResetNetns()
			return fmt.Errorf("move veth to netns: %s %w", strings.TrimSpace(string(out)), err)
		}
	}
	_, _ = netnsExec("ip", "addr", "replace", NSVethCIDR, "dev", VethNS)
	_, _ = netnsExec("ip", "link", "set", VethNS, "up")
	_, _ = run("ip", "addr", "replace", HostVethCIDR, "dev", VethHost)
	_, _ = run("ip", "link", "set", VethHost, "up")
	_, _ = netnsExec("ip", "route", "replace", "default", "via", NSVethGW, "dev", VethNS)
	_, _ = run("sysctl", "-w", "net.ipv4.ip_forward=1")
	// WARP / WireGuard 相关标记路由在部分内核配置下依赖 src_valid_mark=1
	_, _ = run("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1")
	_, _ = netnsExec("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1")
	// NAT：netns 到主 WAN 的建连出口（nft 原生）
	nftEnsureHostWarpUplinkMasq(uplink)
	cleanupLegacyIPTablesNAT(uplink, "")
	st := loadState()
	st.UplinkDev = uplink
	saveState(st)
	return nil
}

func ensureNetnsBypassRules() {
	if !netnsExists() {
		return
	}
	// WARP 会异步重写 cloudflare-warp 链，短时间内可能把 qwp1 放行规则抹掉。
	// 这里做小窗口重试，尽量在链存在后立刻补上。
	for i := 0; i < 6; i++ {
		out, err := netnsExec("nft", "list", "chain", "inet", "cloudflare-warp", "output")
		if err == nil {
			if !strings.Contains(string(out), `oifname "`+VethNS+`" accept`) {
				_, _ = netnsExec("nft", "insert", "rule", "inet", "cloudflare-warp", "output", "oifname", VethNS, "accept")
			}
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
	for i := 0; i < 6; i++ {
		out, err := netnsExec("nft", "list", "chain", "inet", "cloudflare-warp", "input")
		if err == nil {
			if !strings.Contains(string(out), `iifname "`+VethNS+`" accept`) {
				_, _ = netnsExec("nft", "insert", "rule", "inet", "cloudflare-warp", "input", "iifname", VethNS, "accept")
			}
			break
		}
		time.Sleep(120 * time.Millisecond)
	}
}

func enforceNetnsBaseline() {
	if !netnsExists() {
		return
	}
	_, _ = netnsExec("ip", "route", "replace", "default", "via", NSVethGW, "dev", VethNS)
	ensureNetnsBypassRules()
}

func connectWarpWithRecovery() error {
	var last string
	for attempt := 0; attempt < 4; attempt++ {
		enforceNetnsBaseline()
		if out, err := netnsExec(warpCLI, "--accept-tos", "connect"); err != nil {
			last = strings.TrimSpace(string(out))
		}
		if netnsWarpConnectedStable() {
			return nil
		}
		if out, err := netnsExec(warpCLI, "--accept-tos", "status"); err == nil {
			last = strings.TrimSpace(string(out))
		}
		_, _ = netnsExec(warpCLI, "--accept-tos", "disconnect")
		time.Sleep(400 * time.Millisecond)
	}
	if last == "" {
		last = "daemon reports no network"
	}
	return fmt.Errorf("warp connect: %s", last)
}

func netnsWarpConnectedStable() bool {
	consecutive := 0
	for i := 0; i < 60; i++ {
		time.Sleep(250 * time.Millisecond)
		iface := warpIfaceInNetns()
		if warpDaemonReady() && iface != "" {
			consecutive++
			if consecutive >= 16 {
				return true
			}
			continue
		}
		consecutive = 0
	}
	return false
}

// RecoverQuick 在 connect 失败后执行一次人工可复现的最小修复序列。
func RecoverQuick() bool {
	if !netnsExists() {
		return false
	}
	enforceNetnsBaseline()
	_, _ = netnsExec("nft", "insert", "rule", "inet", "cloudflare-warp", "output", "oifname", VethNS, "accept")
	_, _ = netnsExec("nft", "insert", "rule", "inet", "cloudflare-warp", "input", "iifname", VethNS, "accept")
	_, _ = netnsExec(warpCLI, "--accept-tos", "mode", "warp")
	_, _ = netnsExec(warpCLI, "--accept-tos", "connect")
	return netnsWarpConnectedStable()
}

func startSvcInNetns() (int, error) {
	if out, err := netnsExec("pgrep", "-x", "warp-svc"); err == nil && strings.TrimSpace(string(out)) != "" {
		for _, p := range strings.Fields(string(out)) {
			pid, _ := strconv.Atoi(p)
			if pidIsUsable(pid) && warpDaemonReady() {
				return pid, nil
			}
		}
	}
	cmd := exec.Command("ip", "netns", "exec", NetnsName, warpSvc)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, err
	}
	// 等待 IPC socket 就绪
	for i := 0; i < 30; i++ {
		time.Sleep(200 * time.Millisecond)
		if warpDaemonReady() {
			return cmd.Process.Pid, nil
		}
	}
	return cmd.Process.Pid, nil
}

func stopSvcInNetns() {
	if !netnsExists() {
		return
	}
	_, _ = netnsExec("bash", "-lc", "pkill -9 -x warp-svc 2>/dev/null; pkill -9 -x warp-cli 2>/dev/null; true")
	time.Sleep(300 * time.Millisecond)
}

func linkExists(name string) bool {
	if strings.TrimSpace(name) == "" {
		return false
	}
	_, err := run("ip", "link", "show", name)
	return err == nil
}

func deleteLink(name string) {
	if !linkExists(name) {
		return
	}
	_, _ = run("ip", "link", "set", name, "down")
	_, _ = run("ip", "link", "del", name)
}

func removeNATRule(uplink string) {
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if uplink == "" {
		return
	}
	nftRemoveHostWarpUplinkMasq(uplink)
	_ = exec.Command("iptables", "-t", "nat", "-D", "POSTROUTING",
		"-s", "10.99.0.0/30", "-o", uplink, "-j", "MASQUERADE").Run()
}

// ReconcileHostNAT 确保 netns veth 网段到主 WAN 的 NAT 规则存在。
// 说明：qosnat 的 nft 加载会 flush ruleset，可能清掉 iptables-nft 管理的 nat 规则；
// 因此在每次数据面重载后执行一次该校准，避免 netns 失联。
func ReconcileHostNAT() {
	if !netnsExists() {
		return
	}
	st := loadState()
	uplink := strings.TrimSpace(st.UplinkDev)
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if uplink == "" {
		return
	}
	nftEnsureHostWarpUplinkMasq(uplink)
	cleanupLegacyIPTablesNAT(uplink, "")
}

func deleteLinkInNetns(name string) {
	if !netnsExists() || strings.TrimSpace(name) == "" {
		return
	}
	_, _ = netnsExec("ip", "link", "set", name, "down")
	_, _ = netnsExec("ip", "link", "del", name)
}

func deleteWarpLinksInNetns() {
	if !netnsExists() {
		return
	}
	out, err := netnsExec("ip", "-o", "link", "show")
	if err != nil {
		return
	}
	for _, line := range strings.Split(string(out), "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, "warp") && !strings.Contains(low, "wgcf") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			deleteLinkInNetns(strings.TrimSuffix(fields[1], ":"))
		}
	}
}

func ensureNetnsGatewayNAT(warpIface string) {
	if !netnsExists() || strings.TrimSpace(warpIface) == "" {
		return
	}
	_, _ = netnsExec("sysctl", "-w", "net.ipv4.ip_forward=1")
	_, _ = netnsExec("iptables", "-P", "FORWARD", "ACCEPT")
	// 网关模式：来自 qwp1 的转发流量必须在 WARP 出口做 MASQUERADE（nft 原生）。
	nftEnsureNetnsWarpMasq(warpIface)
	cleanupLegacyIPTablesNAT("", warpIface)
	if exec.Command("ip", "netns", "exec", NetnsName, "iptables", "-C", "FORWARD", "-i", VethNS, "-o", warpIface, "-j", "ACCEPT").Run() != nil {
		_, _ = netnsExec("iptables", "-A", "FORWARD", "-i", VethNS, "-o", warpIface, "-j", "ACCEPT")
	}
	if exec.Command("ip", "netns", "exec", NetnsName, "iptables", "-C", "FORWARD", "-i", warpIface, "-o", VethNS, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT").Run() != nil {
		_, _ = netnsExec("iptables", "-A", "FORWARD", "-i", warpIface, "-o", VethNS, "-m", "state", "--state", "RELATED,ESTABLISHED", "-j", "ACCEPT")
	}
}

// Teardown 删除 WARP 隧道、veth 与 netns（断开时完整清理）。
func Teardown() {
	forceResetNetns()
}

func warpIfaceInNetns() string {
	out, err := netnsExec("ip", "-o", "link", "show")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if !strings.Contains(strings.ToLower(line), "warp") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return strings.TrimSuffix(fields[1], ":")
		}
	}
	return ""
}

// Connect 在 netns 内启用 WARP，并将宿主机 qwp0 作为策略路由出口（经 netns 转发到 WARP）。
func Connect() (string, error) {
	StopHostWarpSvc()
	uplink := mainUplinkDev()
	if err := Ensure(uplink); err != nil {
		if needsNetnsReset() {
			forceResetNetns()
			if err2 := Ensure(uplink); err2 != nil {
				return "", err2
			}
		} else {
			return "", err
		}
	}
	pid, err := startSvcInNetns()
	if err != nil {
		return "", fmt.Errorf("start warp-svc in netns: %w", err)
	}
	_, _ = netnsExec(warpCLI, "--accept-tos", "debug", "connectivity-check", "disable")
	if _, err := netnsExec("bash", "-lc", "test -s /var/lib/cloudflare-warp/reg.json"); err != nil {
		_, _ = netnsExec(warpCLI, "--accept-tos", "registration", "new")
	}
	// 在 netns 隔离运行时，mode=warp 在稳定性上明显优于 tunnel_only。
	// 宿主机默认路由不会被接管（策略路由只指向 qwp0）。
	if out, err := netnsExec(warpCLI, "--accept-tos", "mode", "warp"); err != nil {
		return "", fmt.Errorf("warp mode: %s %w", strings.TrimSpace(string(out)), err)
	}
	if err := connectWarpWithRecovery(); err != nil {
		return "", err
	}
	var iface string
	for i := 0; i < 25; i++ {
		time.Sleep(400 * time.Millisecond)
		iface = warpIfaceInNetns()
		if iface != "" {
			break
		}
	}
	if iface == "" {
		return "", fmt.Errorf("warp interface not found in netns")
	}
	// Simplified mode: keep default route via veth gateway (qwp1).
	// WARP will manage its own policy routing/tables inside the namespace.
	_, _ = netnsExec("ip", "route", "replace", "default", "via", NSVethGW, "dev", VethNS)
	ensureNetnsGatewayNAT(iface)
	st := loadState()
	st.SvcPID = pid
	st.HostIface = VethHost
	st.Connected = true
	st.UplinkDev = uplink
	saveState(st)
	return VethHost, nil
}

// Disconnect 断开 WARP，并删除 netns / veth / 隧道接口。
func Disconnect() {
	Teardown()
}

// HostInterface 返回已移到宿主机的 WARP 隧道接口名。
func HostInterface() string {
	st := loadState()
	if st.HostIface != "" {
		if _, err := run("ip", "link", "show", st.HostIface); err == nil {
			return st.HostIface
		}
	}
	return detectHostWarpIface()
}

func detectHostWarpIface() string {
	out, err := run("ip", "-o", "link", "show")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		low := strings.ToLower(line)
		if !strings.Contains(low, "warp") && !strings.Contains(low, "wgcf") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			return strings.TrimSuffix(fields[1], ":")
		}
	}
	return ""
}

// IsConnected 报告 WARP 是否在 netns 中已连接且隧道在宿主机可用。
func IsConnected() bool {
	if !netnsExists() {
		return false
	}
	st := loadState()
	if !st.Connected {
		return false
	}
	iface := HostInterface()
	if iface == "" {
		return false
	}
	out, err := netnsExec(warpCLI, "--accept-tos", "status")
	if err != nil {
		return false
	}
	low := strings.ToLower(string(out))
	return strings.Contains(low, "connected") && !strings.Contains(low, "disconnected")
}

// ServiceRunning  netns 内 warp-svc 是否在运行。
func ServiceRunning() bool {
	if !netnsExists() {
		return false
	}
	out, err := netnsExec("pgrep", "-x", "warp-svc")
	if err != nil || strings.TrimSpace(string(out)) == "" {
		return false
	}
	for _, p := range strings.Fields(string(out)) {
		pid, _ := strconv.Atoi(p)
		if pidIsUsable(pid) {
			return true
		}
	}
	return false
}
