package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

const (
	warpInstallStateIdle    = "idle"
	warpInstallStateRunning = "running"
	warpInstallStateOK      = "ok"
	warpInstallStateFailed  = "failed"

	warpInstallStatusFile = "/var/lib/qosnat2/warp-install-status.json"
	warpInstallLogFile    = "/var/lib/qosnat2/warp-install.log"
)

type warpInstallStatus struct {
	State      string `json:"state"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
}

var (
	warpInstallMu      sync.Mutex
	warpInstallRunning bool
)

func (srv *Server) handleNetworkWarpStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	warpnetns.Reconcile()
	srv.reconcileWarpStoreState()
	installed := commandExists("warp-cli")
	netnsHealthy := warpnetns.NetnsHealthy()
	service := warpnetns.ServiceRunning()
	statusOut := ""
	connected := warpnetns.IsConnected()
	if connected {
		if out, err := exec.Command("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status").CombinedOutput(); err == nil {
			statusOut = strings.TrimSpace(string(out))
		}
	} else if installed && !netnsHealthy {
		// 仅 netns 模式未启用时展示宿主机 warp-cli（避免损坏 netns 时误报已连接）。
		if out, err := exec.Command("warp-cli", "--accept-tos", "status").CombinedOutput(); err == nil {
			statusOut = strings.TrimSpace(string(out))
		}
	}
	iface := warpnetns.HostInterface()
	if iface == "" {
		iface = detectWarpInterface()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"installed":     installed,
		"service_up":    service,
		"connected":     connected,
		"netns_healthy": netnsHealthy,
		"interface":     iface,
		"status_raw":    statusOut,
		"root":          os.Getuid() == 0,
		"install_job":   getWarpInstallStatus(),
	})
}

func (srv *Server) handleNetworkWarpInstallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, getWarpInstallStatus())
}

func (srv *Server) handleNetworkWarpInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "install requires root"})
		return
	}
	if warpInstallComplete() {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "warp already installed",
			"job": warpInstallStatus{
				State:   warpInstallStateOK,
				Message: "already installed",
			},
		})
		return
	}
	if err := startWarpInstallAsync(r, srv); err != nil {
		writeJSON(w, http.StatusConflict, map[string]any{"error": err.Error(), "job": getWarpInstallStatus()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "WARP install started in background",
		"job":     getWarpInstallStatus(),
	})
}

func startWarpInstallAsync(r *http.Request, srv *Server) error {
	warpInstallMu.Lock()
	if warpInstallRunning {
		warpInstallMu.Unlock()
		return fmt.Errorf("install already running")
	}
	warpInstallRunning = true
	warpInstallMu.Unlock()

	go func() {
		defer func() {
			warpInstallMu.Lock()
			warpInstallRunning = false
			warpInstallMu.Unlock()
		}()
		started := time.Now().UTC().Format(time.RFC3339)
		saveWarpInstallStatus(warpInstallStatus{
			State:     warpInstallStateRunning,
			Message:   "running apt install",
			StartedAt: started,
		})

		_ = os.MkdirAll(filepath.Dir(warpInstallLogFile), 0755)
		logf, err := os.Create(warpInstallLogFile)
		if err != nil {
			finishWarpInstall(started, warpInstallStateFailed, err.Error())
			return
		}
		defer logf.Close()

		codename := ""
		if out, err := exec.Command("lsb_release", "-cs").CombinedOutput(); err == nil {
			codename = strings.TrimSpace(string(out))
		}
		if codename == "" {
			codename = "jammy"
		}
		repoLine := fmt.Sprintf("deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ %s main", codename)

		run := func(args ...string) error {
			return runDebCmd(logf, args...)
		}
		if err := run("bash", "-lc", "curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor --output /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg"); err != nil {
			finishWarpInstall(started, warpInstallStateFailed, err.Error())
			return
		}
		if err := run("bash", "-lc", "echo '"+repoLine+"' > /etc/apt/sources.list.d/cloudflare-client.list"); err != nil {
			finishWarpInstall(started, warpInstallStateFailed, err.Error())
			return
		}
		if err := waitForDPKGLock(120 * time.Second); err != nil {
			finishWarpInstall(started, warpInstallStateFailed, err.Error())
			return
		}
		if err := run(append(aptNoninteractiveArgs("-o", "DPkg::Lock::Timeout=120"), "update")...); err != nil {
			finishWarpInstall(started, warpInstallStateFailed, summarizeAptFailure("apt update", err))
			return
		}
		repairWarpDebState(logf)
		cleanBrokenWarpPackage(logf)
		installErr := run(append(aptNoninteractiveArgs(), "install", "-y", "cloudflare-warp")...)
		if installErr != nil || !warpPackageConfigured() {
			repairWarpDebState(logf)
			cleanBrokenWarpPackage(logf)
			installErr = run(append(aptNoninteractiveArgs(), "install", "-y", "cloudflare-warp")...)
		}
		if (installErr != nil || !warpPackageConfigured()) && !warpInstallComplete() {
			msg := "cloudflare-warp 未正确安装（依赖缺失）。请执行: apt --fix-broken install"
			if installErr != nil {
				msg = summarizeAptFailure("cloudflare-warp install", installErr)
			}
			finishWarpInstall(started, warpInstallStateFailed, msg)
			return
		}
		srv.auditLog(r, "network.warp.install", codename)
		finishWarpInstall(started, warpInstallStateOK, "install completed")
	}()
	return nil
}

func getWarpInstallStatus() warpInstallStatus {
	warpInstallMu.Lock()
	defer warpInstallMu.Unlock()
	st := loadWarpInstallStatus()
	if warpInstallComplete() && st.State == warpInstallStateFailed {
		st.State = warpInstallStateOK
		if st.Message == "" || strings.Contains(st.Message, "exit status") || strings.Contains(st.Message, "fix-broken") {
			st.Message = "install completed"
		}
	}
	if warpInstallRunning {
		st.State = warpInstallStateRunning
		if st.Message == "" {
			st.Message = "install in progress"
		}
	}
	return st
}

func loadWarpInstallStatus() warpInstallStatus {
	st := warpInstallStatus{State: warpInstallStateIdle}
	b, err := os.ReadFile(warpInstallStatusFile)
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if st.State == "" {
		st.State = warpInstallStateIdle
	}
	if b, err := os.ReadFile(warpInstallLogFile); err == nil {
		st.LogTail = tailLines(string(b), 80)
	}
	return st
}

func saveWarpInstallStatus(st warpInstallStatus) {
	_ = os.MkdirAll(filepath.Dir(warpInstallStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(warpInstallStatusFile, b, 0600)
}

func finishWarpInstall(started, state, msg string) {
	st := warpInstallStatus{
		State:      state,
		Message:    msg,
		StartedAt:  started,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if b, err := os.ReadFile(warpInstallLogFile); err == nil {
		st.LogTail = tailLines(string(b), 80)
	}
	saveWarpInstallStatus(st)
}

// warpDebDpkgRepairPrefixes 可用 download+dpkg -i 修复的依赖包（不含 cloudflare-warp 本体）。
var warpDebDpkgRepairPrefixes = []string{
	"libgtk-3",
	"libwebkit2gtk",
	"libayatana",
	"libdbusmenu-gtk3",
	"xdg-desktop-portal-gtk",
}

// warpDebPrefixes 用于检测 WARP 相关未配置包（含 cloudflare-warp）。
var warpDebPrefixes = append([]string{"cloudflare-warp"}, warpDebDpkgRepairPrefixes...)

func isWarpDebPackage(name string) bool {
	for _, p := range warpDebPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func isWarpDebDpkgRepairPackage(name string) bool {
	for _, p := range warpDebDpkgRepairPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func warpPackageConfigured() bool {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Status}", "cloudflare-warp").Output()
	return err == nil && strings.TrimSpace(string(out)) == "install ok installed"
}

func warpInstallComplete() bool {
	return warpPackageConfigured() && commandExists("warp-cli")
}

// cleanBrokenWarpPackage 移除半安装/未配置的 cloudflare-warp，以便 apt 重新拉取完整依赖。
func cleanBrokenWarpPackage(log io.Writer) {
	out, err := exec.Command("dpkg-query", "-W", "-f=${Status}", "cloudflare-warp").Output()
	if err != nil {
		return
	}
	st := strings.TrimSpace(string(out))
	if st == "install ok installed" {
		return
	}
	_, _ = fmt.Fprintf(log, "\n--- clean: removing broken cloudflare-warp (status=%s) ---\n", st)
	_ = runDebCmd(log, "dpkg", "--remove", "--force-depends", "cloudflare-warp")
}

func aptNoninteractiveArgs(extra ...string) []string {
	base := []string{
		"apt-get",
		"-o", "Dpkg::Options::=--force-confdef",
		"-o", "Dpkg::Options::=--force-confold",
		"-o", "DPkg::Lock::Timeout=300",
	}
	return append(base, extra...)
}

func runDebCmd(log io.Writer, args ...string) error {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
	cmd.Stdout = log
	cmd.Stderr = log
	return cmd.Run()
}

// repairWarpDebState 仅修复 WARP 相关半安装/未配置/reinst-required 包。
func repairWarpDebState(log io.Writer) {
	out, _ := exec.Command("dpkg", "-l").Output()
	var reinst, unconfigured []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		st, pkg := fields[0], fields[1]
		if !isWarpDebDpkgRepairPackage(pkg) {
			continue
		}
		if strings.HasPrefix(st, "r") {
			reinst = append(reinst, pkg)
		}
		if len(st) >= 2 && (st[1] == 'U' || st[1] == 'F') {
			unconfigured = append(unconfigured, pkg)
		}
	}
	for _, pkg := range reinst {
		_, _ = fmt.Fprintf(log, "\n--- repair: apt-get download %s ---\n", pkg)
		dir, err := os.MkdirTemp("", "qosnat2-deb-repair-")
		if err != nil {
			continue
		}
		dl := exec.Command("apt-get", "download", pkg)
		dl.Dir = dir
		dl.Env = append(os.Environ(), "DEBIAN_FRONTEND=noninteractive")
		dl.Stdout = log
		dl.Stderr = log
		if dl.Run() != nil {
			_ = os.RemoveAll(dir)
			continue
		}
		debs, _ := filepath.Glob(filepath.Join(dir, "*.deb"))
		for _, deb := range debs {
			_, _ = fmt.Fprintf(log, "--- repair: dpkg -i %s ---\n", deb)
			_ = runDebCmd(log, "dpkg", "--force-confold", "-i", deb)
		}
		_ = os.RemoveAll(dir)
	}
	all := append(append([]string{}, reinst...), unconfigured...)
	seen := map[string]bool{}
	var toConfigure []string
	for _, p := range all {
		if !seen[p] {
			seen[p] = true
			toConfigure = append(toConfigure, p)
		}
	}
	if len(toConfigure) > 0 {
		_, _ = fmt.Fprintf(log, "\n--- repair: dpkg --configure %s ---\n", strings.Join(toConfigure, " "))
		_ = runDebCmd(log, append([]string{"dpkg", "--force-confold", "--configure"}, toConfigure...)...)
	}
	cleanBrokenWarpPackage(log)
}

// summarizeAptFailure 从 apt/dpkg 错误中提取可读摘要（优先 “not configured yet” 类提示）
func summarizeAptFailure(step string, err error) string {
	if err == nil {
		return step + " failed"
	}
	msg := strings.TrimSpace(err.Error())
	if strings.Contains(msg, "not configured yet") {
		return step + ": 系统存在未完成的软件包配置（已尝试 dpkg --configure -a）。请查看安装日志，或手动执行: sudo dpkg --configure -a && sudo apt-get -f install -y"
	}
	if strings.Contains(msg, "exit status 100") {
		return step + ": apt 依赖/配置错误 (exit 100)，请查看下方安装日志"
	}
	return step + ": " + msg
}

func tailLines(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= max {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-max:], "\n")
}

func cmdOutput(args ...string) string {
	if len(args) == 0 {
		return ""
	}
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err != nil {
		if s == "" {
			return err.Error()
		}
		return s + " (" + err.Error() + ")"
	}
	return s
}

func warpConnectedFromStatus(raw string) bool {
	low := strings.ToLower(strings.TrimSpace(raw))
	return strings.Contains(low, "connected") && !strings.Contains(low, "disconnected") && !strings.Contains(low, "no network")
}

func waitWarpHealthyStable(samples int, interval time.Duration, needConsecutive int) (bool, string) {
	last := ""
	okStreak := 0
	for i := 0; i < samples; i++ {
		last = cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status")
		if warpConnectedFromStatus(last) {
			okStreak++
			if okStreak >= needConsecutive {
				return true, last
			}
		} else {
			okStreak = 0
		}
		time.Sleep(interval)
	}
	return false, last
}

func collectWarpConnectDiagnostics() map[string]any {
	diag := map[string]any{
		"netns":                warpnetns.NetnsName,
		"netns_exists":         strings.Contains(cmdOutput("ip", "netns", "list"), warpnetns.NetnsName),
		"netns_healthy":        warpnetns.NetnsHealthy(),
		"service_running":      warpnetns.ServiceRunning(),
		"connected":            warpnetns.IsConnected(),
		"host_iface":           warpnetns.HostInterface(),
		"netns_warp_status":    cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status"),
		"netns_links":          cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "ip", "-br", "link"),
		"netns_routes_v4":      cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "ip", "-4", "route"),
		"netns_nft_output":     cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "nft", "list", "chain", "inet", "cloudflare-warp", "output"),
		"netns_nft_input":      cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "nft", "list", "chain", "inet", "cloudflare-warp", "input"),
		"host_route_table_202": cmdOutput("ip", "-4", "route", "show", "table", "202"),
	}
	return diag
}

func (srv *Server) handleNetworkWarpConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "connect requires root"})
		return
	}
	if !commandExists("warp-cli") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "warp not installed"})
		return
	}
	warpnetns.Reconcile()
	iface, err := warpnetns.Connect()
	if err != nil {
		if !warpnetns.RecoverQuick() {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":       err.Error(),
				"diagnostics": collectWarpConnectDiagnostics(),
			})
			return
		}
		if !warpnetns.IsConnected() {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":       err.Error(),
				"diagnostics": collectWarpConnectDiagnostics(),
			})
			return
		}
		iface = warpnetns.HostInterface()
		if iface == "" {
			iface = "qwp0"
		}
	}
	statusNow := cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status")
	if !warpConnectedFromStatus(statusNow) {
		if warpnetns.RecoverQuick() {
			iface = warpnetns.HostInterface()
			if iface == "" {
				iface = "qwp0"
			}
			statusNow = cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status")
			if !warpConnectedFromStatus(statusNow) {
				writeJSON(w, http.StatusInternalServerError, map[string]any{
					"error":       "warp recover completed but tunnel is still unhealthy",
					"diagnostics": collectWarpConnectDiagnostics(),
				})
				return
			}
		} else {
			writeJSON(w, http.StatusInternalServerError, map[string]any{
				"error":       "warp connect reported success but tunnel is not healthy",
				"diagnostics": collectWarpConnectDiagnostics(),
			})
			return
		}
	}
	if stable, finalStatus := waitWarpHealthyStable(8, 1*time.Second, 3); !stable {
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error": "warp connected transiently but did not remain healthy",
			"diagnostics": func() map[string]any {
				d := collectWarpConnectDiagnostics()
				d["final_status"] = finalStatus
				return d
			}(),
		})
		return
	} else {
		statusNow = finalStatus
	}
	_ = restoreRoutesAfterWarpConnect(srv)
	if err := srv.applyWarpWanLink(iface); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "warp connected but wan link sync failed: " + err.Error()})
		return
	}
	if err := warpnetns.ReconcileAfterWanLink(); err != nil || !warpnetns.IsConnected() {
		warpnetns.ResetBroken()
		_ = srv.removeWarpWanLink()
		msg := "warp netns broken after wan link sync"
		if err != nil {
			msg += ": " + err.Error()
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{
			"error":       msg,
			"diagnostics": collectWarpConnectDiagnostics(),
		})
		return
	}
	statusNow = cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status")
	srv.auditLog(r, "network.warp.connect", iface)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"interface": iface,
		"netns":     warpnetns.NetnsName,
		"wan_link":  store.WarpWanLink(iface),
		"message":   "WARP 已在隔离网络命名空间中连接，主路由未改变",
		"health": map[string]any{
			"connected":       warpConnectedFromStatus(statusNow),
			"service_running": warpnetns.ServiceRunning(),
			"netns_status":    statusNow,
		},
	})
}

func (srv *Server) handleNetworkWarpDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "disconnect requires root"})
		return
	}
	if !commandExists("warp-cli") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "warp not installed"})
		return
	}
	warpnetns.Disconnect()
	_ = restoreRoutesAfterWarpConnect(srv)
	_ = srv.removeWarpWanLink()
	warpnetns.Reconcile()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func serviceActive(name string) bool {
	out, err := exec.Command("systemctl", "is-active", name).CombinedOutput()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

func detectWarpInterface() string {
	out, err := exec.Command("bash", "-lc", "ip -o link show | awk -F': ' '{print $2}' | sed 's/@.*//'").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, l := range strings.Split(string(out), "\n") {
		n := strings.TrimSpace(l)
		low := strings.ToLower(n)
		if strings.Contains(low, "warp") || low == "wgcf" {
			return n
		}
	}
	return ""
}

func waitForDPKGLock(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if !dpkgLockHeld() {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("dpkg lock still held after %s; please retry later", timeout.String())
		}
		time.Sleep(2 * time.Second)
	}
}

func dpkgLockHeld() bool {
	paths := []string{"/var/lib/dpkg/lock-frontend", "/var/lib/dpkg/lock"}
	for _, p := range paths {
		out, err := exec.Command("bash", "-lc", "fuser "+p).CombinedOutput()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return true
		}
	}
	return false
}
