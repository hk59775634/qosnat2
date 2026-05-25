package ocserv

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	installStateIdle    = "idle"
	installStateRunning = "running"
	installStateOK      = "ok"
	installStateFailed  = "failed"

	installStatusFile = "/var/lib/qosnat2/ocserv-install-status.json"
	installLogFile    = "/var/lib/qosnat2/ocserv-install.log"
)

// InstallJobStatus 源码安装任务状态（供 UI 轮询）
type InstallJobStatus struct {
	State      string `json:"state"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
	Script     string `json:"script,omitempty"`
}

var (
	installMu     sync.Mutex
	installRunning bool
)

func loadInstallStatus() InstallJobStatus {
	st := InstallJobStatus{State: installStateIdle}
	b, err := os.ReadFile(installStatusFile)
	if err != nil {
		return st
	}
	_ = json.Unmarshal(b, &st)
	if st.State == "" {
		st.State = installStateIdle
	}
	if b, err := os.ReadFile(installLogFile); err == nil {
		st.LogTail = tailLines(string(b), 80)
	}
	return st
}

func saveInstallStatus(st InstallJobStatus) {
	_ = os.MkdirAll(filepath.Dir(installStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(installStatusFile, b, 0644)
}

func tailLines(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= max {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-max:], "\n")
}

// GetInstallStatus 返回最近一次安装任务状态
func GetInstallStatus() InstallJobStatus {
	installMu.Lock()
	defer installMu.Unlock()
	st := loadInstallStatus()
	if installRunning {
		st.State = installStateRunning
		st.Message = "install in progress"
	}
	return st
}

// StartInstallAsync 后台执行安装脚本（单实例）
func StartInstallAsync(script string) error {
	installMu.Lock()
	if installRunning {
		installMu.Unlock()
		return errInstallBusy
	}
	installRunning = true
	installMu.Unlock()

	go func() {
		defer func() {
			installMu.Lock()
			installRunning = false
			installMu.Unlock()
		}()

		started := time.Now().UTC().Format(time.RFC3339)
		st := InstallJobStatus{
			State:     installStateRunning,
			Message:   "running install script",
			StartedAt: started,
			Script:    script,
		}
		saveInstallStatus(st)

		_ = os.MkdirAll(filepath.Dir(installLogFile), 0755)
		logf, err := os.Create(installLogFile)
		if err != nil {
			finishInstall(started, installStateFailed, err.Error(), "")
			return
		}
		defer logf.Close()

		cmd := exec.Command("bash", script)
		cmd.Stdout = logf
		cmd.Stderr = logf
		runErr := cmd.Run()
		logf.Close()

		logBody, _ := os.ReadFile(installLogFile)
		tail := tailLines(string(logBody), 80)
		if runErr != nil {
			finishInstall(started, installStateFailed, runErr.Error(), tail)
			return
		}
		finishInstall(started, installStateOK, "install completed", tail)
	}()
	return nil
}

func finishInstall(started, state, msg, tail string) {
	st := InstallJobStatus{
		State:      state,
		Message:    msg,
		StartedAt:  started,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
		LogTail:    tail,
	}
	saveInstallStatus(st)
}

type installBusyError struct{}

func (installBusyError) Error() string { return "install already running" }

var errInstallBusy = installBusyError{}
