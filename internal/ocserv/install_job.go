package ocserv

import (
	"encoding/json"
	"fmt"
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

	installStatusFile    = "/var/lib/qosnat2/ocserv-install-status.json"
	installLogFile       = "/var/lib/qosnat2/ocserv-install.log"
	ocservReleaseTagFile = "/var/lib/qosnat2/ocserv-release-tag"
	defaultSourceTag     = "1.4.2"
)

type installBusyError struct{}

func (installBusyError) Error() string { return "install already running" }

var errInstallBusy = installBusyError{}

func saveInstallStatus(st InstallJobStatus) {
	_ = os.MkdirAll(filepath.Dir(installStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(installStatusFile, b, 0600)
}

func tailLines(s string, max int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) <= max {
		return strings.Join(lines, "\n")
	}
	return strings.Join(lines[len(lines)-max:], "\n")
}

// InstallJobStatus 安装任务状态（供 UI 轮询）
type InstallJobStatus struct {
	State      string `json:"state"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
	Script     string `json:"script,omitempty"`
	Method     string `json:"method,omitempty"`
	Version    string `json:"version,omitempty"`
}

var (
	installMu      sync.Mutex
	installRunning bool
)

// GetInstallStatus 读取安装任务状态
func GetInstallStatus() InstallJobStatus {
	installMu.Lock()
	defer installMu.Unlock()
	b, err := os.ReadFile(installStatusFile)
	if err != nil {
		return InstallJobStatus{State: installStateIdle}
	}
	var st InstallJobStatus
	if json.Unmarshal(b, &st) != nil {
		return InstallJobStatus{State: installStateIdle}
	}
	if installRunning && st.State != installStateRunning {
		st.State = installStateRunning
		st.Message = "install in progress"
	}
	return st
}

// StartInstallAsync 后台执行源码编译安装（单实例）。
func StartInstallAsync(method, version string) error {
	installMu.Lock()
	if installRunning {
		installMu.Unlock()
		return errInstallBusy
	}
	installRunning = true
	installMu.Unlock()

	method = strings.TrimSpace(strings.ToLower(method))
	if method == "" || method == "release" {
		method = "source"
	}
	if method != "source" {
		installMu.Lock()
		installRunning = false
		installMu.Unlock()
		return fmt.Errorf("unsupported install method: %s (only source)", method)
	}
	version = strings.TrimSpace(version)
	version = strings.TrimPrefix(version, "ocserv-")
	version = strings.TrimPrefix(version, "v")
	if version == "" {
		version = defaultSourceTag
	}
	script := InstallScriptPath()

	go func() {
		defer func() {
			installMu.Lock()
			installRunning = false
			installMu.Unlock()
		}()

		started := time.Now().UTC().Format(time.RFC3339)
		st := InstallJobStatus{
			State:     installStateRunning,
			Message:   "running install",
			StartedAt: started,
			Script:    script,
			Method:    method,
			Version:   version,
		}
		saveInstallStatus(st)

		_ = os.MkdirAll(filepath.Dir(installLogFile), 0755)
		logf, err := os.Create(installLogFile)
		if err != nil {
			finishInstall(started, installStateFailed, err.Error(), "")
			return
		}
		defer logf.Close()

		cmd := exec.Command("bash", script, "--method", "source", "--version", version)
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
