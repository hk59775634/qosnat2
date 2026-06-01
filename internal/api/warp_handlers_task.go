package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

const (
	warpTaskOpConnect    = "connect"
	warpTaskOpDisconnect = "disconnect"

	warpTaskStatusFile = "/var/lib/qosnat2/warp-task-status.json"
	warpTaskTimeout = 90 * time.Second
)

type warpTaskStatus struct {
	Op         string         `json:"op,omitempty"`
	State      string         `json:"state"`
	Message    string         `json:"message,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
}

var (
	warpTaskMu      sync.Mutex
	warpTaskRunning bool
)

func loadWarpTaskStatus() warpTaskStatus {
	st := warpTaskStatus{State: warpInstallStateIdle}
	b, err := os.ReadFile(warpTaskStatusFile)
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if st.State == "" {
		st.State = warpInstallStateIdle
	}
	return st
}

func saveWarpTaskStatus(st warpTaskStatus) {
	_ = os.MkdirAll(filepath.Dir(warpTaskStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(warpTaskStatusFile, b, 0600)
}

func finishWarpTask(started, op, state, msg string, result map[string]any) {
	st := warpTaskStatus{
		Op:         op,
		State:      state,
		Message:    msg,
		StartedAt:  started,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Result:     result,
	}
	saveWarpTaskStatus(st)
}

func expireStaleWarpTaskIfNeeded(st warpTaskStatus) warpTaskStatus {
	if st.State != warpInstallStateRunning || st.StartedAt == "" || warpTaskRunning {
		return st
	}
	t, err := time.Parse(time.RFC3339, st.StartedAt)
	if err != nil || time.Since(t) < warpTaskTimeout {
		return st
	}
	finishWarpTask(st.StartedAt, st.Op, warpInstallStateFailed, "warp task expired (stale status)", nil)
	return loadWarpTaskStatus()
}

func getWarpTaskStatus() warpTaskStatus {
	warpTaskMu.Lock()
	defer warpTaskMu.Unlock()
	st := loadWarpTaskStatus()
	st = expireStaleWarpTaskIfNeeded(st)
	if warpTaskRunning && st.State != warpInstallStateRunning {
		st.State = warpInstallStateRunning
	}
	return st
}

// clearStaleWarpTaskOnBoot 服务重启后清除残留的 running 任务状态，避免 UI 永久卡在连接中。
func clearStaleWarpTaskOnBoot() {
	st := loadWarpTaskStatus()
	if st.State != warpInstallStateRunning {
		return
	}
	finishWarpTask(st.StartedAt, st.Op, warpInstallStateFailed, "interrupted by qosnatd restart", nil)
}

func (srv *Server) startWarpTask(op string, r *http.Request, run func() (map[string]any, error)) error {
	warpTaskMu.Lock()
	if warpTaskRunning {
		warpTaskMu.Unlock()
		return fmt.Errorf("warp %s already running", op)
	}
	warpTaskRunning = true
	warpTaskMu.Unlock()

	go func() {
		defer func() {
			warpTaskMu.Lock()
			warpTaskRunning = false
			warpTaskMu.Unlock()
		}()
		started := time.Now().UTC().Format(time.RFC3339)
		saveWarpTaskStatus(warpTaskStatus{
			Op:        op,
			State:     warpInstallStateRunning,
			Message:   "running",
			StartedAt: started,
		})
		deadline := time.After(warpTaskTimeout)
		type taskResult struct {
			result map[string]any
			err    error
		}
		ch := make(chan taskResult, 1)
		go func() {
			r, e := run()
			ch <- taskResult{r, e}
		}()
		var result map[string]any
		var err error
		select {
		case tr := <-ch:
			result, err = tr.result, tr.err
		case <-deadline:
			if op == warpTaskOpConnect {
				warpnetns.ScrubAfterFailedConnect()
			}
			err = fmt.Errorf("warp %s timed out after %s", op, warpTaskTimeout)
		}
		if err != nil {
			if result == nil {
				result = map[string]any{}
			}
			if op == warpTaskOpConnect {
				if _, ok := result["diagnostics"]; !ok {
					result["diagnostics"] = collectWarpConnectDiagnostics()
				}
			}
			finishWarpTask(started, op, warpInstallStateFailed, err.Error(), result)
			return
		}
		finishWarpTask(started, op, warpInstallStateOK, "completed", result)
		if op == warpTaskOpConnect {
			iface, _ := result["interface"].(string)
			if r != nil {
				srv.auditLog(r, "network.warp.connect", iface)
			}
		} else if r != nil {
			srv.auditLog(r, "network.warp.disconnect", "")
		}
	}()
	return nil
}

func (srv *Server) handleNetworkWarpTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, getWarpTaskStatus())
}

func (srv *Server) handleNetworkWarpConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "connect requires root")
		return
	}
	if !commandExists("warp-cli") {
		writeBadRequest(w, "warp not installed")
		return
	}
	if err := srv.startWarpConnectAsync(r); err != nil {
		writeConflictWithExtra(w, err.Error(), map[string]any{"job": getWarpTaskStatus()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "WARP connect started in background",
		"job":     getWarpTaskStatus(),
	})
}

func (srv *Server) startWarpConnectAsync(r *http.Request) error {
	return srv.startWarpTask(warpTaskOpConnect, r, func() (map[string]any, error) {
		return srv.runWarpConnect()
	})
}

func isWarpLicenseError(err error) bool {
	return err != nil && strings.Contains(err.Error(), "warp license:")
}

func (srv *Server) runWarpConnect() (map[string]any, error) {
	warpnetns.BeginOp()
	defer warpnetns.EndOp()

	_ = srv.store.Update(func(st *store.State) {
		store.SetWarpEnabled(st, true)
	})
	if err := srv.store.Save(); err != nil {
		return nil, err
	}

	if warpnetns.NeedsReset() {
		warpnetns.ResetBroken()
	}
	licenseKey := strings.TrimSpace(srv.store.Get().Network.WarpLicenseKey)
	iface, err := warpnetns.Connect(licenseKey)
	if err != nil {
		if isWarpLicenseError(err) {
			_ = srv.store.Update(func(st *store.State) {
				store.SetWarpEnabled(st, false)
			})
			_ = srv.store.Save()
			warpnetns.ScrubAfterFailedConnect()
			return nil, fmt.Errorf("%s", err.Error())
		}
		if !warpnetns.RecoverQuick() {
			warpnetns.ScrubAfterFailedConnect()
			return nil, fmt.Errorf("%s", err.Error())
		}
		iface = warpnetns.HostInterface()
		if iface == "" {
			iface = "qwp0"
		}
	}
	if iface == "" {
		iface = warpnetns.HostInterface()
	}
	if iface == "" {
		iface = "qwp0"
	}
	statusNow := cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status")
	if !warpConnectedFromStatus(statusNow) {
		warpnetns.ScrubAfterFailedConnect()
		return nil, fmt.Errorf("warp connect finished but cli status is not connected: %s", statusNow)
	}
	stable, finalStatus := waitWarpConnectedStable(4, 500*time.Millisecond, 2)
	if !stable {
		warpnetns.ScrubAfterFailedConnect()
		return map[string]any{"final_status": finalStatus},
			fmt.Errorf("warp connected transiently but did not remain connected")
	}
	statusNow = finalStatus
	policyWarn := ""
	if err := srv.applyWarpPoliciesAfterConnect(iface); err != nil {
		policyWarn = err.Error()
		log.Printf("warp connect: policy apply: %v", err)
	} else if err := verifyWarpTunnelHealthy(); err != nil {
		policyWarn = err.Error()
		log.Printf("warp connect: post-policy health: %v", err)
	}
	statusNow = cmdOutput("ip", "netns", "exec", warpnetns.NetnsName, "warp-cli", "--accept-tos", "status")
	if !warpConnectedFromStatus(statusNow) {
		warpnetns.ScrubAfterFailedConnect()
		return nil, fmt.Errorf("warp tunnel lost after policy apply: %s", statusNow)
	}
	warpnetns.ClearExitInfoCache()
	msg := "WARP 已在隔离网络命名空间中连接，主路由未改变"
	if policyWarn != "" {
		msg = msg + "（策略应用警告: " + policyWarn + "）"
	}
	return map[string]any{
		"ok":        true,
		"interface": iface,
		"netns":     warpnetns.NetnsName,
		"wan_link":  store.WarpWanLink(iface),
		"message":   msg,
		"health": map[string]any{
			"connected":       warpConnectedFromStatus(statusNow),
			"service_running": warpnetns.ServiceRunning(),
			"netns_status":    statusNow,
		},
	}, nil
}

func (srv *Server) handleNetworkWarpDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "disconnect requires root")
		return
	}
	if !commandExists("warp-cli") {
		writeBadRequest(w, "warp not installed")
		return
	}
	// 立即持久化关闭意图，避免看门狗/状态轮询在异步清理完成前自动重连。
	if err := srv.store.Update(func(st *store.State) {
		store.SetWarpEnabled(st, false)
	}); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if err := srv.store.Save(); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if err := srv.startWarpDisconnectAsync(r); err != nil {
		writeConflictWithExtra(w, err.Error(), map[string]any{"job": getWarpTaskStatus()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "WARP disconnect started in background",
		"job":     getWarpTaskStatus(),
	})
}

func (srv *Server) startWarpDisconnectAsync(r *http.Request) error {
	return srv.startWarpTask(warpTaskOpDisconnect, r, func() (map[string]any, error) {
		return srv.runWarpDisconnect()
	})
}

func (srv *Server) runWarpDisconnect() (map[string]any, error) {
	warpnetns.BeginOp()
	defer warpnetns.EndOp()
	warpnetns.ClearExitInfoCache()
	_ = srv.store.Update(func(st *store.State) {
		store.SetWarpEnabled(st, false)
	})
	if err := srv.store.Save(); err != nil {
		return nil, err
	}
	if !store.WarpLicenseKeyConfigured(srv.store.Get()) {
		warpnetns.DeleteRegistration()
	}
	warpnetns.Disconnect()
	_ = restoreRoutesAfterWarpConnect(srv)
	_ = srv.removeWarpWanLink()
	warpnetns.Reconcile()
	return map[string]any{"ok": true}, nil
}
