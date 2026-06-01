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
	"sync/atomic"
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

var warpOpInFlight int32

// BeginOp 标记 WARP 连接/断开/恢复正在进行；此期间 Reconcile 不得重置 netns。
func BeginOp() {
	atomic.AddInt32(&warpOpInFlight, 1)
}

// EndOp 结束 WARP 操作标记。
func EndOp() {
	atomic.AddInt32(&warpOpInFlight, -1)
}

// OpActive 是否有 WARP 连接/断开/恢复正在进行。
func OpActive() bool {
	return atomic.LoadInt32(&warpOpInFlight) > 0
}

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

func netnsPinPath() string {
	return filepath.Join("/var/run/netns", NetnsName)
}

func netnsExists() bool {
	_, err := os.Stat(netnsPinPath())
	return err == nil
}

// NetnsExists reports whether the qosnat2-warp network namespace mount is present.
func NetnsExists() bool {
	return netnsExists()
}

// netnsUsable netns 存在且 ip netns exec 可用（排除 Peer netns reference is invalid 等损坏状态）。
func netnsUsable() bool {
	if !netnsExists() || !netnsPinValid() {
		return false
	}
	if hostVethPeerBroken() {
		return false
	}
	_, err := netnsExec("true")
	return err == nil
}

func netnsExecUsable() bool {
	if !netnsExists() {
		return false
	}
	_, err := run("ip", "netns", "exec", NetnsName, "true")
	return err == nil
}

func netnsPinValid() bool {
	return netnsExecUsable()
}

// forceRemoveNetnsPin 强制拆除 netns 挂载点（损坏 pin / 只读空文件时 ip netns delete 可能无效）。
func forceRemoveNetnsPin() {
	if !netnsExists() {
		return
	}
	killProcsInNamedNetns()
	for i := 0; i < 5; i++ {
		_, _ = run("ip", "netns", "delete", NetnsName)
		if !netnsExists() {
			return
		}
		time.Sleep(150 * time.Millisecond)
	}
	pin := netnsPinPath()
	_ = os.Chmod(pin, 0600)
	_, _ = run("umount", "-l", pin)
	_ = os.Remove(pin)
	if netnsExists() {
		_, _ = run("umount", "-f", pin)
		_ = os.Chmod(pin, 0600)
		_ = os.Remove(pin)
	}
}

// clearStaleNetnsPin 移除损坏或不可 exec 的 netns pin，避免 ip netns add 报 File exists。
func clearStaleNetnsPin() {
	if !netnsExists() {
		return
	}
	if netnsExecUsable() {
		return
	}
	forceRemoveNetnsPin()
}

func hostVethPeerBroken() bool {
	if !linkExists(VethHost) {
		return false
	}
	out, _ := run("ip", "-d", "-o", "link", "show", VethHost)
	if !strings.Contains(string(out), "link-netnsid 0") {
		return false
	}
	// link-netnsid 0 仅表示对端 netns 引用失效；若 netns 内 qwp1 仍在则勿判损坏。
	if netnsUsable() {
		_, err := netnsExec("ip", "link", "show", VethNS)
		return err != nil
	}
	return true
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

// NetnsHealthy netns 可 exec 且 veth 成对可用。
func NetnsHealthy() bool {
	return netnsUsable() && vethPairHealthy()
}

// NeedsReset 检测孤儿 veth 或损坏 netns，连接前应强制清理。
func NeedsReset() bool {
	return needsNetnsReset()
}

// needsNetnsReset 检测孤儿 veth 或 netns 损坏（常见于 WARP 连接中断后）。
func needsNetnsReset() bool {
	hasNetns := netnsExists()
	hasVeth := linkExists(VethHost)
	if hasVeth && !hasNetns {
		return true
	}
	if hasNetns && !netnsUsable() {
		return true
	}
	if hasNetns && !hasVeth {
		return true
	}
	if hasVeth && hasNetns && !vethPairHealthy() {
		return true
	}
	return false
}

// PrepareForConnect 连接前强制完整清理并重建 netns/veth（避免孤儿 qwp0 残留）。
func PrepareForConnect() error {
	if netnsExists() && !netnsExecUsable() {
		forceRemoveNetnsPin()
	}
	RestoreHostResolv()
	ensureNetnsResolvFile()
	scrubWarpStack()
	uplink := mainUplinkDev()
	if uplink == "" {
		return fmt.Errorf("cannot detect main WAN device for warp netns uplink")
	}
	return Ensure(uplink)
}

// ScrubAfterFailedConnect 连接失败后清理，便于 UI 再次点击连接。
func ScrubAfterFailedConnect() {
	scrubWarpStack()
	RestoreHostResolv()
}

func clearConnectedState() {
	st := loadState()
	if !st.Connected && st.SvcPID == 0 && st.HostIface == "" {
		return
	}
	saveState(State{Netns: NetnsName, UplinkDev: st.UplinkDev})
}

// ResetBroken 对外暴露的损坏 netns 强制清理（断开/重连失败时调用）。
func ResetBroken() {
	scrubWarpStack()
}

// scrubWarpStack 无条件拆除 WARP netns/veth/进程，恢复干净初始状态。
func scrubWarpStack() {
	st := loadState()
	uplink := strings.TrimSpace(st.UplinkDev)
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	StopHostWarpSvc()
	_, _ = run("bash", "-lc", "pkill -9 -x warp-svc 2>/dev/null; pkill -9 -x warp-cli 2>/dev/null; true")
	time.Sleep(200 * time.Millisecond)
	// 损坏 netns 时先删 veth，否则 ip netns 子系统全局报错且无法 exec。
	if !netnsUsable() {
		forceDeleteLink(VethHost)
		forceDeleteLink(VethNS)
		clearStaleNetnsPin()
	} else {
		stopSvcInNetns()
		deleteWarpLinksInNetns()
		forceDeleteLink(VethHost)
		forceDeleteLink(VethNS)
	}
	removeNATRule(uplink)
	forceDeleteNetns()
	clearStaleNetnsPin()
	saveState(State{Netns: NetnsName, UplinkDev: uplink})
}

// forceResetNetns 强制拆除损坏的 netns/veth，便于 Ensure 干净重建。
func forceResetNetns() {
	scrubWarpStack()
}

// forceDeleteNetns 删除 netns 挂载点；损坏状态下 ip link del 后重试。
func forceDeleteNetns() {
	if !netnsExists() {
		return
	}
	killProcsInNamedNetns()
	for i := 0; i < 5; i++ {
		forceDeleteLink(VethHost)
		forceDeleteLink(VethNS)
		if _, err := run("ip", "netns", "delete", NetnsName); err == nil {
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	forceRemoveNetnsPin()
	forceDeleteLink(VethHost)
	forceDeleteLink(VethNS)
}

func killProcsInNamedNetns() {
	pinStat, err := os.Stat(netnsPinPath())
	if err != nil {
		return
	}
	pinInfo, ok := pinStat.Sys().(*syscall.Stat_t)
	if !ok {
		return
	}
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(e.Name())
		if err != nil {
			continue
		}
		procStat, err := os.Stat(filepath.Join("/proc", e.Name(), "ns/net"))
		if err != nil {
			continue
		}
		procInfo, ok := procStat.Sys().(*syscall.Stat_t)
		if !ok || procInfo.Dev != pinInfo.Dev || procInfo.Ino != pinInfo.Ino {
			continue
		}
		_, _ = exec.Command("kill", "-9", strconv.Itoa(pid)).CombinedOutput()
	}
	time.Sleep(200 * time.Millisecond)
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
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if uplink == "" {
		return fmt.Errorf("cannot detect main WAN device for warp netns uplink")
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if needsNetnsReset() || (netnsExists() && !netnsUsable()) {
			forceResetNetns()
		}
		lastErr = ensureNetnsVeth(uplink)
		if lastErr == nil {
			return nil
		}
		forceResetNetns()
	}
	return lastErr
}

func recreateNetnsPin() error {
	forceRemoveNetnsPin()
	out, err := run("ip", "netns", "add", NetnsName)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "File exists") || strings.Contains(err.Error(), "exists") {
			forceRemoveNetnsPin()
			out, err = run("ip", "netns", "add", NetnsName)
			msg = strings.TrimSpace(string(out))
		}
		if err != nil {
			forceRemoveNetnsPin()
			return fmt.Errorf("ip netns add: %s %w", msg, err)
		}
	}
	if !netnsExecUsable() {
		forceRemoveNetnsPin()
		return fmt.Errorf("warp netns %q exists but is not usable", NetnsName)
	}
	return nil
}

func ensureNetnsVeth(uplink string) error {
	if err := recreateNetnsPin(); err != nil {
		return err
	}
	_, _ = netnsExec("ip", "link", "set", "lo", "up")
	if ifaceSysfsExists(VethHost) {
		if _, err := netnsExec("ip", "link", "show", VethNS); err != nil {
			forceDeleteLink(VethHost)
		}
	}
	if !ifaceSysfsExists(VethHost) {
		if out, err := run("ip", "link", "add", VethHost, "type", "veth", "peer", "name", VethNS); err != nil {
			return fmt.Errorf("veth: %s %w", strings.TrimSpace(string(out)), err)
		}
	}
	if _, err := netnsExec("ip", "link", "show", VethNS); err != nil {
		if out, err := run("ip", "link", "set", VethNS, "netns", NetnsName); err != nil {
			return fmt.Errorf("move veth to netns: %s %w", strings.TrimSpace(string(out)), err)
		}
	}
	_, _ = netnsExec("ip", "addr", "replace", NSVethCIDR, "dev", VethNS)
	_, _ = netnsExec("ip", "link", "set", VethNS, "up")
	_, _ = run("ip", "addr", "replace", HostVethCIDR, "dev", VethHost)
	_, _ = run("ip", "link", "set", VethHost, "up")
	_, _ = netnsExec("ip", "route", "replace", "default", "via", NSVethGW, "dev", VethNS)
	_, _ = run("sysctl", "-w", "net.ipv4.ip_forward=1")
	_, _ = run("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1")
	_, _ = netnsExec("sysctl", "-w", "net.ipv4.conf.all.src_valid_mark=1")
	nftEnsureHostWarpUplinkMasq(uplink)
	cleanupLegacyIPTablesNAT(uplink, "")
	st := loadState()
	st.UplinkDev = uplink
	saveState(st)
	return nil
}

func restoreVethIfBroken(uplink string) error {
	if !hostVethPeerBroken() && vethPairHealthy() {
		return nil
	}
	forceDeleteLink(VethHost)
	forceDeleteLink(VethNS)
	if needsNetnsReset() {
		forceResetNetns()
	}
	return Ensure(uplink)
}

func ensureNetnsBypassRules() {
	if !netnsUsable() {
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
	if !netnsUsable() {
		return
	}
	_, _ = netnsExec("ip", "route", "replace", "default", "via", NSVethGW, "dev", VethNS)
	ensureNetnsBypassRules()
}

const (
	warpConnectCLIAttempts = 2
	warpStablePollInterval = 250 * time.Millisecond
	warpStablePollMax      = 16 // ~4s per CLI connect attempt
	warpStableNeedHits     = 3  // ~750ms consecutive connected
)

func connectWarpWithRecovery() error {
	var last string
	for attempt := 0; attempt < warpConnectCLIAttempts; attempt++ {
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
		if attempt+1 < warpConnectCLIAttempts {
			_, _ = netnsExec(warpCLI, "--accept-tos", "disconnect")
			time.Sleep(300 * time.Millisecond)
		}
	}
	if last == "" {
		last = "daemon reports no network"
	}
	return fmt.Errorf("warp connect: %s", last)
}

func netnsWarpConnectedStable() bool {
	consecutive := 0
	for i := 0; i < warpStablePollMax; i++ {
		time.Sleep(warpStablePollInterval)
		iface := warpIfaceInNetns()
		if out, err := netnsExec(warpCLI, "--accept-tos", "status"); err == nil && WarpStatusConnected(string(out)) && iface != "" {
			consecutive++
			if consecutive >= warpStableNeedHits {
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
	if err := PrepareForConnect(); err != nil {
		return false
	}
	if _, err := startSvcInNetns(); err != nil {
		scrubWarpStack()
		return false
	}
	enforceNetnsBaseline()
	_, _ = netnsExec("nft", "insert", "rule", "inet", "cloudflare-warp", "output", "oifname", VethNS, "accept")
	_, _ = netnsExec("nft", "insert", "rule", "inet", "cloudflare-warp", "input", "iifname", VethNS, "accept")
	_, _ = netnsExec(warpCLI, "--accept-tos", "debug", "connectivity-check", "disable")
	_, _ = netnsExec(warpCLI, "--accept-tos", "mode", "warp")
	_, _ = netnsExec(warpCLI, "--accept-tos", "connect")
	ok := netnsWarpConnectedStable()
	if !ok {
		scrubWarpStack()
	}
	return ok
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
	cmd := exec.Command("ip", warpSvcStartArgs()...)
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
	return ifaceSysfsExists(name)
}

func ifaceSysfsExists(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	_, err := os.Stat(filepath.Join("/sys/class/net", name))
	return err == nil
}

func forceDeleteLink(name string) {
	if !ifaceSysfsExists(name) {
		return
	}
	_, _ = run("ip", "link", "set", name, "down")
	if _, err := run("ip", "link", "del", name); err == nil {
		return
	}
	// 对端 netns 已损坏时，先移入 init netns 再删。
	_, _ = run("ip", "link", "set", name, "netns", "1")
	_, _ = run("ip", "link", "del", name)
}

func deleteLink(name string) {
	forceDeleteLink(name)
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

// connectedStackRecoverable WARP 已连接或 netns 内 warp-svc 仍在运行，应尽量修复而非拆除 netns。
func connectedStackRecoverable() bool {
	if OpActive() {
		return true
	}
	if !netnsUsable() {
		return false
	}
	st := loadState()
	if linkExists(VethHost) || st.Connected {
		return ServiceRunning() || st.Connected
	}
	return false
}

// TryRepairConnectedNetns 在策略/nft 重载后尝试修复 veth/netns，避免误删 qosnat2-warp。
func TryRepairConnectedNetns() error {
	if OpActive() {
		return nil
	}
	st := loadState()
	uplink := strings.TrimSpace(st.UplinkDev)
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if err := restoreVethIfBroken(uplink); err != nil {
		return err
	}
	enforceNetnsBaseline()
	return nil
}

// Reconcile 检测并修复损坏 netns，flush ruleset 后回补 NAT/bypass，并清理虚假 connected 状态。
func Reconcile() {
	if OpActive() {
		EnsureHostNATOnly()
		return
	}
	preserve := connectedStackRecoverable()
	if !preserve {
		StopHostWarpSvc()
		RestoreHostResolv()
	}
	if needsNetnsReset() {
		if preserve {
			_ = TryRepairConnectedNetns()
			EnsureHostNATOnly()
			return
		}
		forceResetNetns()
		return
	}
	st := loadState()
	if st.Connected && !NetnsHealthy() {
		clearConnectedState()
		return
	}
	if !netnsUsable() {
		if st.Connected {
			clearConnectedState()
		}
		return
	}
	uplink := strings.TrimSpace(st.UplinkDev)
	if uplink == "" {
		uplink = mainUplinkDev()
	}
	if uplink != "" {
		nftEnsureHostWarpUplinkMasq(uplink)
		cleanupLegacyIPTablesNAT(uplink, "")
	}
	if st.Connected || linkExists(VethHost) {
		ensureNetnsBypassRules()
	}
}

// ReconcileAfterWanLink 在 applyWarpWanLink 后校验 netns 仍可用。
func ReconcileAfterWanLink() error {
	if OpActive() {
		EnsureHostNATOnly()
		return nil
	}
	EnsureHostNATOnly()
	if needsNetnsReset() {
		if connectedStackRecoverable() {
			if err := TryRepairConnectedNetns(); err != nil {
				return fmt.Errorf("warp netns repair after wan link sync: %w", err)
			}
			if !NetnsHealthy() {
				return fmt.Errorf("warp netns unhealthy after wan link sync (repaired)")
			}
			return nil
		}
		scrubWarpStack()
		return fmt.Errorf("warp netns broken after wan link sync")
	}
	if !NetnsHealthy() {
		if connectedStackRecoverable() {
			if err := TryRepairConnectedNetns(); err == nil && NetnsHealthy() {
				return nil
			}
		}
		clearConnectedState()
		return fmt.Errorf("warp netns unhealthy after wan link sync")
	}
	return nil
}

// EnsureHostNATOnly 仅回补宿主机 NAT/bypass 规则，不触发 netns 重置（WARP 已连接时）。
func EnsureHostNATOnly() {
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
	if netnsUsable() {
		ensureNetnsBypassRules()
	}
}

// ReconcileHostNAT 确保 netns veth 网段到主 WAN 的 NAT 规则存在。
// 说明：qosnat 的 nft 加载会 flush ruleset，可能清掉 iptables-nft 管理的 nat 规则；
// 因此在每次数据面重载后执行一次该校准，避免 netns 失联。
func ReconcileHostNAT() {
	if IsConnected() || ServiceRunning() || linkExists(VethHost) {
		EnsureHostNATOnly()
		return
	}
	Reconcile()
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

// Teardown 删除 WARP 隧道、veth 与 netns。
func Teardown() {
	scrubWarpStack()
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

func applyWarpLicense(licenseKey string) error {
	licenseKey = strings.TrimSpace(licenseKey)
	if licenseKey == "" {
		return nil
	}
	out, err := netnsExec(warpCLI, "--accept-tos", "registration", "license", licenseKey)
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			msg = err.Error()
		}
		return fmt.Errorf("warp license: %s", msg)
	}
	return nil
}

// ApplyLicense runs warp-cli registration license in the WARP netns (WARP+ upgrade; same as warp-go --update --license).
func ApplyLicense(licenseKey string) error {
	if !NetnsHealthy() {
		return fmt.Errorf("warp netns is not healthy")
	}
	if err := applyWarpLicense(licenseKey); err != nil {
		return err
	}
	ClearExitInfoCache()
	return nil
}

// DeleteRegistration 清除 netns 内 warp-cli 注册（无 License 时便于重新 registration new）。
func DeleteRegistration() {
	if !NetnsHealthy() {
		return
	}
	_, _ = netnsExec(warpCLI, "--accept-tos", "registration", "delete")
}

// Connect 在 netns 内启用 WARP，并将宿主机 qwp0 作为策略路由出口（经 netns 转发到 WARP）。
// licenseKey 非空时通过 warp-cli registration license 激活 WARP+。
func Connect(licenseKey string) (string, error) {
	if err := PrepareForConnect(); err != nil {
		return "", err
	}
	uplink := mainUplinkDev()
	pid, err := startSvcInNetns()
	if err != nil {
		return "", fmt.Errorf("start warp-svc in netns: %w", err)
	}
	_, _ = netnsExec(warpCLI, "--accept-tos", "debug", "connectivity-check", "disable")
	if _, err := netnsExec("bash", "-lc", "test -s /var/lib/cloudflare-warp/reg.json"); err != nil {
		_, _ = netnsExec(warpCLI, "--accept-tos", "registration", "new")
	}
	if err := applyWarpLicense(licenseKey); err != nil {
		return "", err
	}
	// 在 netns 隔离运行时，mode=warp 在稳定性上明显优于 tunnel_only。
	// 宿主机默认路由不会被接管（策略路由只指向 qwp0）。
	if out, err := netnsExec(warpCLI, "--accept-tos", "mode", "warp"); err != nil {
		return "", fmt.Errorf("warp mode: %s %w", strings.TrimSpace(string(out)), err)
	}
	if err := connectWarpWithRecovery(); err != nil {
		return "", err
	}
	if err := restoreVethIfBroken(uplink); err != nil {
		return "", fmt.Errorf("restore veth after warp connect: %w", err)
	}
	var iface string
	for i := 0; i < 15; i++ {
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
	if err := restoreVethIfBroken(uplink); err != nil {
		return "", fmt.Errorf("restore veth after warp gateway nat: %w", err)
	}
	enforceNetnsBaseline()
	if !netnsHealthyAfterConnect() {
		return "", fmt.Errorf("warp netns unhealthy after tunnel up")
	}
	st := loadState()
	st.SvcPID = pid
	st.HostIface = VethHost
	st.Connected = true
	st.UplinkDev = uplink
	saveState(st)
	return VethHost, nil
}

func netnsHealthyAfterConnect() bool {
	for i := 0; i < 6; i++ {
		if NetnsHealthy() {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return NetnsHealthy()
}

// Disconnect 断开 WARP 并完整清理 netns/veth。
func Disconnect() {
	RestoreHostResolv()
	scrubWarpStack()
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
	if linkExists(VethHost) {
		return VethHost
	}
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
			return linkNameFromField(fields[1])
		}
	}
	return ""
}

func linkNameFromField(raw string) string {
	raw = strings.TrimSuffix(strings.TrimSpace(raw), ":")
	if i := strings.Index(raw, "@"); i >= 0 {
		raw = raw[:i]
	}
	return raw
}

// IsConnected 报告 WARP 是否在 netns 中已连接（以运行时探测为准，不单独依赖 state 文件）。
func IsConnected() bool {
	return probeConnectedRuntime()
}

// ServiceRunning  netns 内 warp-svc 是否在运行。
func ServiceRunning() bool {
	if !netnsUsable() {
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
