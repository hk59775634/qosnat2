package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/frr"
	"github.com/hk59775634/qosnat2/internal/route"
)

const (
	frrInstallStatusFile = "/var/lib/qosnat2/frr-install-status.json"
	frrInstallLogFile    = "/var/lib/qosnat2/frr-install.log"
)

type frrInstallStatus struct {
	State      string `json:"state"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
}

var (
	frrInstallMu      sync.Mutex
	frrInstallRunning bool
)

func getFrrInstallStatus() frrInstallStatus {
	st := frrInstallStatus{State: warpInstallStateIdle}
	b, err := os.ReadFile(frrInstallStatusFile)
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if st.State == "" {
		st.State = warpInstallStateIdle
	}
	if st.State == warpInstallStateRunning && !frrInstallRunning {
		if b, err := os.ReadFile(frrInstallLogFile); err == nil {
			st.LogTail = tailLines(string(b), 40)
		}
	}
	return st
}

func saveFrrInstallStatus(st frrInstallStatus) {
	_ = os.MkdirAll(filepath.Dir(frrInstallStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(frrInstallStatusFile, b, 0600)
}

func finishFrrInstall(started, state, msg string) {
	st := frrInstallStatus{
		State:      state,
		Message:    msg,
		StartedAt:  started,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if b, err := os.ReadFile(frrInstallLogFile); err == nil {
		st.LogTail = tailLines(string(b), 80)
	}
	saveFrrInstallStatus(st)
}

func (srv *Server) handleFRRInstallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, getFrrInstallStatus())
}

func (srv *Server) handleFRRInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "安装 FRR 需要 root 运行 qosnatd")
		return
	}
	if frr.PackageInstalled() {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "frr already installed",
			"job": frrInstallStatus{
				State:   warpInstallStateOK,
				Message: "already installed",
			},
		})
		return
	}
	if err := srv.startFrrInstallAsync(r); err != nil {
		writeConflictWithExtra(w, err.Error(), map[string]any{"job": getFrrInstallStatus()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "FRR 安装已在后台开始",
		"job":     getFrrInstallStatus(),
	})
}

func (srv *Server) startFrrInstallAsync(r *http.Request) error {
	frrInstallMu.Lock()
	if frrInstallRunning {
		frrInstallMu.Unlock()
		return fmt.Errorf("frr install already running")
	}
	frrInstallRunning = true
	frrInstallMu.Unlock()

	go func() {
		defer func() {
			frrInstallMu.Lock()
			frrInstallRunning = false
			frrInstallMu.Unlock()
		}()
		started := time.Now().UTC().Format(time.RFC3339)
		saveFrrInstallStatus(frrInstallStatus{
			State:     warpInstallStateRunning,
			Message:   "installing frr package",
			StartedAt: started,
		})
		_ = os.MkdirAll(filepath.Dir(frrInstallLogFile), 0755)
		logf, err := os.Create(frrInstallLogFile)
		if err != nil {
			finishFrrInstall(started, warpInstallStateFailed, err.Error())
			return
		}
		defer logf.Close()
		_, _ = fmt.Fprintf(logf, "=== install-frr %s ===\n", started)

		script := frr.InstallScriptPath()
		if st, err := os.Stat(script); err != nil || st.IsDir() {
			finishFrrInstall(started, warpInstallStateFailed, "missing "+script)
			return
		}
		cmd := exec.Command("bash", script)
		cmd.Stdout = logf
		cmd.Stderr = logf
		if err := cmd.Run(); err != nil {
			finishFrrInstall(started, warpInstallStateFailed, "install failed: "+err.Error())
			return
		}
		st := srv.store.Get()
		if st.System.FrrBootOnStartup {
			_ = frr.SetBootEnabled(true)
		}
		if err := frr.PrepareInstalled(); err != nil {
			log.Printf("frr install: prepare config: %v", err)
		}
		if _, err := route.ApplyFromState(st); err != nil {
			log.Printf("frr install: route apply: %v", err)
		}
		srv.auditLog(r, "frr.install", "")
		finishFrrInstall(started, warpInstallStateOK, "frr installed")
	}()
	return nil
}
