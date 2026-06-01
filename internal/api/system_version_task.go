package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/releasecatalog"
)

const (
	versionSwitchStatusFileVar = "/var/lib/qosnat2/version-switch-status.json"
)

var versionSwitchStatusFile = versionSwitchStatusFileVar

const versionSwitchStaleTimeout = 20 * time.Minute

type versionSwitchStatus struct {
	State      string         `json:"state"`
	Message    string         `json:"message,omitempty"`
	TargetTag  string         `json:"target_tag,omitempty"`
	StartedAt  string         `json:"started_at,omitempty"`
	FinishedAt string         `json:"finished_at,omitempty"`
	Result     map[string]any `json:"result,omitempty"`
}

var (
	versionSwitchMu      sync.Mutex
	versionSwitchRunning bool
)

func loadVersionSwitchStatus() versionSwitchStatus {
	st := versionSwitchStatus{State: warpInstallStateIdle}
	b, err := os.ReadFile(versionSwitchStatusFile)
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if st.State == "" {
		st.State = warpInstallStateIdle
	}
	return st
}

func saveVersionSwitchStatus(st versionSwitchStatus) {
	_ = os.MkdirAll(filepath.Dir(versionSwitchStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(versionSwitchStatusFile, b, 0600)
}

func patchVersionSwitchStatus(patch func(*versionSwitchStatus)) {
	st := loadVersionSwitchStatus()
	patch(&st)
	saveVersionSwitchStatus(st)
}

func finishVersionSwitch(started, state, msg, targetTag string, result map[string]any) {
	st := versionSwitchStatus{
		State:      state,
		Message:    msg,
		TargetTag:  targetTag,
		StartedAt:  started,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		Result:     result,
	}
	saveVersionSwitchStatus(st)
}

func reconcileVersionSwitchStatusLocked() {
	st := loadVersionSwitchStatus()
	if st.State != warpInstallStateRunning || versionSwitchRunning {
		return
	}
	target := releasecatalog.NormalizeID(st.TargetTag)
	current := releasecatalog.NormalizeID(readTextFile(qosnatReleaseTag))
	if target != "" && target == current {
		finishVersionSwitch(st.StartedAt, warpInstallStateOK, "upgrade completed", target, map[string]any{
			"ok":      true,
			"tag":     target,
			"message": "版本切换完成",
		})
		return
	}
	msg := "version switch interrupted; please retry"
	if st.StartedAt != "" {
		if started, err := time.Parse(time.RFC3339, st.StartedAt); err == nil && time.Since(started) > versionSwitchStaleTimeout {
			msg = "version switch timed out; please retry"
		}
	}
	finishVersionSwitch(st.StartedAt, warpInstallStateFailed, msg, target, nil)
}

func clearVersionSwitchStatusLocked() {
	saveVersionSwitchStatus(versionSwitchStatus{State: warpInstallStateIdle})
}

func getVersionSwitchStatus() versionSwitchStatus {
	versionSwitchMu.Lock()
	defer versionSwitchMu.Unlock()
	reconcileVersionSwitchStatusLocked()
	st := loadVersionSwitchStatus()
	if versionSwitchRunning && st.State != warpInstallStateRunning {
		st.State = warpInstallStateRunning
	}
	return st
}

func (srv *Server) startVersionSwitchAsync(r *http.Request, versionID, downloadRoute string) error {
	versionSwitchMu.Lock()
	reconcileVersionSwitchStatusLocked()
	if versionSwitchRunning {
		versionSwitchMu.Unlock()
		return fmt.Errorf("version switch already running")
	}
	if st := loadVersionSwitchStatus(); st.State == warpInstallStateRunning {
		versionSwitchMu.Unlock()
		return fmt.Errorf("version switch already running")
	}
	versionSwitchRunning = true
	versionSwitchMu.Unlock()

	go func() {
		defer func() {
			versionSwitchMu.Lock()
			versionSwitchRunning = false
			versionSwitchMu.Unlock()
		}()
		started := time.Now().UTC().Format(time.RFC3339)
		saveVersionSwitchStatus(versionSwitchStatus{
			State:     warpInstallStateRunning,
			Message:   "preparing download route",
			TargetTag: versionID,
			StartedAt: started,
		})

		st := srv.store.Get()
		tempRules, err := applyVersionSwitchTempEgress(st, downloadRoute)
		if err != nil {
			finishVersionSwitch(started, warpInstallStateFailed, err.Error(), versionID, map[string]any{
				"download_route": downloadRoute,
			})
			return
		}
		defer removeVersionSwitchTempEgress(tempRules)

		patchVersionSwitchStatus(func(st *versionSwitchStatus) {
			st.Message = "downloading release (" + downloadRoute + ")"
		})
		if err := releasecatalog.InstallReleaseBinary(versionID, qosnatBinPath, downloadRoute); err != nil {
			finishVersionSwitch(started, warpInstallStateFailed, err.Error(), versionID, map[string]any{
				"download_route": downloadRoute,
			})
			return
		}
		patchVersionSwitchStatus(func(st *versionSwitchStatus) {
			st.Message = "writing release tag"
		})
		_ = os.MkdirAll(filepath.Dir(qosnatReleaseTag), 0755)
		if err := os.WriteFile(qosnatReleaseTag, []byte(versionID+"\n"), 0644); err != nil {
			finishVersionSwitch(started, warpInstallStateFailed, "write release tag: "+err.Error(), versionID, nil)
			return
		}
		patchVersionSwitchStatus(func(st *versionSwitchStatus) {
			st.Message = "restarting qosnatd"
		})
		srv.auditLog(r, "system.version.switch", versionID)
		finishVersionSwitch(started, warpInstallStateOK, "upgrade completed, service is restarting", versionID, map[string]any{
			"ok":             true,
			"tag":            versionID,
			"download_route": downloadRoute,
			"message":        "版本切换完成，服务即将重启",
		})
		cmd := exec.Command("bash", "-lc", "sleep 1; systemctl restart qosnatd.service")
		_ = cmd.Start()
	}()
	return nil
}

func (srv *Server) handleSystemVersionSwitchReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	versionSwitchMu.Lock()
	defer versionSwitchMu.Unlock()
	if versionSwitchRunning {
		writeConflictWithExtra(w, "version switch already running", map[string]any{"job": loadVersionSwitchStatus()})
		return
	}
	clearVersionSwitchStatusLocked()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "job": loadVersionSwitchStatus()})
}

func (srv *Server) handleSystemVersionSwitchStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, getVersionSwitchStatus())
}
