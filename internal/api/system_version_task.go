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

const versionSwitchStatusFile = "/var/lib/qosnat2/version-switch-status.json"

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

func getVersionSwitchStatus() versionSwitchStatus {
	versionSwitchMu.Lock()
	defer versionSwitchMu.Unlock()
	st := loadVersionSwitchStatus()
	if versionSwitchRunning && st.State != warpInstallStateRunning {
		st.State = warpInstallStateRunning
	}
	return st
}

func (srv *Server) startVersionSwitchAsync(r *http.Request, versionID string) error {
	versionSwitchMu.Lock()
	if versionSwitchRunning {
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
			Message:   "downloading release",
			TargetTag: versionID,
			StartedAt: started,
		})

		if err := releasecatalog.InstallReleaseBinary(versionID, qosnatBinPath); err != nil {
			finishVersionSwitch(started, warpInstallStateFailed, err.Error(), versionID, nil)
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
		cmd := exec.Command("bash", "-lc", "sleep 1; systemctl restart qosnatd.service")
		if err := cmd.Start(); err != nil {
			finishVersionSwitch(started, warpInstallStateFailed, "restart qosnatd: "+err.Error(), versionID, map[string]any{
				"tag": versionID,
			})
			return
		}
		srv.auditLog(r, "system.version.switch", versionID)
		finishVersionSwitch(started, warpInstallStateOK, "upgrade completed, service is restarting", versionID, map[string]any{
			"ok":      true,
			"tag":     versionID,
			"message": "版本切换完成，服务即将重启",
		})
	}()
	return nil
}

func (srv *Server) handleSystemVersionSwitchStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, getVersionSwitchStatus())
}
