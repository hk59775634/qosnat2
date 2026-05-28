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

	"github.com/hk59775634/qosnat2/internal/releasecatalog"
)

const (
	installStateIdle    = "idle"
	installStateRunning = "running"
	installStateOK      = "ok"
	installStateFailed  = "failed"

	installStatusFile = "/var/lib/qosnat2/ocserv-install-status.json"
	installLogFile    = "/var/lib/qosnat2/ocserv-install.log"
)

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
	_ = os.WriteFile(installStatusFile, b, 0600)
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

// StartInstallAsync 后台执行安装（单实例）。method: release（默认）或 source（仅开发构建）。
func StartInstallAsync(method, version string) error {
	installMu.Lock()
	if installRunning {
		installMu.Unlock()
		return errInstallBusy
	}
	installRunning = true
	installMu.Unlock()

	method = strings.TrimSpace(strings.ToLower(method))
	if method == "" {
		method = "release"
	}
	version = releasecatalog.NormalizeOcservVersion(version)
	if version == "" && method == "release" {
		if entries, err := releasecatalog.ListEntries("ocserv"); err == nil && len(entries) > 0 {
			version = releasecatalog.NormalizeOcservVersion(entries[0].ID)
			if version == "" {
				version = releasecatalog.NormalizeOcservVersion(entries[0].Tag)
			}
		}
	}
	if version == "" && method == "source" {
		version = "1.4.2"
	}
	if method == "release" && version == "" {
		installMu.Lock()
		installRunning = false
		installMu.Unlock()
		return fmt.Errorf("release install requires version (manifest empty?)")
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

		var cmd *exec.Cmd
		if method == "source" {
			cmd = exec.Command("bash", script, "--method", "source", "--version", version)
		} else {
			url := ocservReleaseDownloadURL(version)
			cmd = exec.Command("bash", script, "--method", "release", "--version", version, "--url", url)
		}
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
		saveOcservReleaseTag(version)
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
