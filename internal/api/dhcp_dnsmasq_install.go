package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/releasecatalog"
	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	dnsmasqChnroutesInstallStatusFile = "/var/lib/qosnat2/dnsmasq-chnroutes-install-status.json"
	dnsmasqChnroutesInstallLogFile    = "/var/lib/qosnat2/dnsmasq-chnroutes-install.log"
)

type dnsmasqChnroutesInstallStatus struct {
	State      string `json:"state"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
}

var (
	dnsmasqChnroutesInstallMu      sync.Mutex
	dnsmasqChnroutesInstallRunning bool
)

func getDnsmasqChnroutesInstallStatus() dnsmasqChnroutesInstallStatus {
	st := dnsmasqChnroutesInstallStatus{State: warpInstallStateIdle}
	b, err := os.ReadFile(dnsmasqChnroutesInstallStatusFile)
	if err == nil {
		_ = json.Unmarshal(b, &st)
	}
	if st.State == "" {
		st.State = warpInstallStateIdle
	}
	if st.State == warpInstallStateRunning && !dnsmasqChnroutesInstallRunning {
		if b, err := os.ReadFile(dnsmasqChnroutesInstallLogFile); err == nil {
			st.LogTail = tailLines(string(b), 40)
		}
	}
	return st
}

func saveDnsmasqChnroutesInstallStatus(st dnsmasqChnroutesInstallStatus) {
	_ = os.MkdirAll(filepath.Dir(dnsmasqChnroutesInstallStatusFile), 0755)
	b, _ := json.Marshal(st)
	_ = os.WriteFile(dnsmasqChnroutesInstallStatusFile, b, 0600)
}

func finishDnsmasqChnroutesInstall(started, state, msg string) {
	st := dnsmasqChnroutesInstallStatus{
		State:      state,
		Message:    msg,
		StartedAt:  started,
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	}
	if b, err := os.ReadFile(dnsmasqChnroutesInstallLogFile); err == nil {
		st.LogTail = tailLines(string(b), 80)
	}
	saveDnsmasqChnroutesInstallStatus(st)
}

func resolveChnroutesBuildScript() (string, error) {
	var roots []string
	for _, c := range []string{
		os.Getenv("QOSNAT_ROOT"),
		"/opt/qosnat2",
	} {
		c = strings.TrimSpace(c)
		if c != "" {
			roots = append(roots, c)
		}
	}
	if wr := strings.TrimSpace(os.Getenv("WEB_ROOT")); wr != "" {
		roots = append(roots, filepath.Clean(filepath.Join(wr, "..", "..")))
	}
	seen := map[string]bool{}
	for _, root := range roots {
		root = filepath.Clean(root)
		if root == "" || seen[root] {
			continue
		}
		seen[root] = true
		p := filepath.Join(root, "scripts", "build-dnsmasq-chnroutes.sh")
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 scripts/build-dnsmasq-chnroutes.sh（需要 /opt/qosnat2 源码目录）")
}

func (srv *Server) handleDHCPDnsmasqInstallChnroutesStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, getDnsmasqChnroutesInstallStatus())
}

func (srv *Server) handleDHCPDnsmasqInstallChnroutes(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "安装 patched dnsmasq 需要 root 运行 qosnatd")
		return
	}
	if dnsmasq.SupportsChnroutes() {
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"message": "dnsmasq 已支持 chnroutes",
			"job": dnsmasqChnroutesInstallStatus{
				State:   warpInstallStateOK,
				Message: "already installed",
			},
		})
		return
	}
	if err := srv.startDnsmasqChnroutesInstallAsync(r); err != nil {
		writeConflictWithExtra(w, err.Error(), map[string]any{"job": getDnsmasqChnroutesInstallStatus()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "dnsmasq chnroutes 安装已在后台开始（优先使用 release 预编译包）",
		"job":     getDnsmasqChnroutesInstallStatus(),
	})
}

func (srv *Server) startDnsmasqChnroutesInstallAsync(r *http.Request) error {
	dnsmasqChnroutesInstallMu.Lock()
	if dnsmasqChnroutesInstallRunning {
		dnsmasqChnroutesInstallMu.Unlock()
		return fmt.Errorf("dnsmasq chnroutes install already running")
	}
	dnsmasqChnroutesInstallRunning = true
	dnsmasqChnroutesInstallMu.Unlock()

	go func() {
		defer func() {
			dnsmasqChnroutesInstallMu.Lock()
			dnsmasqChnroutesInstallRunning = false
			dnsmasqChnroutesInstallMu.Unlock()
		}()
		started := time.Now().UTC().Format(time.RFC3339)
		_ = os.MkdirAll(filepath.Dir(dnsmasqChnroutesInstallLogFile), 0755)
		logf, err := os.Create(dnsmasqChnroutesInstallLogFile)
		if err != nil {
			finishDnsmasqChnroutesInstall(started, warpInstallStateFailed, err.Error())
			return
		}
		defer logf.Close()
		_, _ = fmt.Fprintf(logf, "=== dnsmasq-chnroutes install %s ===\n", started)

		if ok, msg := installDnsmasqChnroutesPrebuilt(logf, started); ok {
			if msg != "" {
				_, _ = fmt.Fprintln(logf, msg)
			}
		} else if ok, msg := installDnsmasqChnroutesFromRelease(logf, started); ok {
			if msg != "" {
				_, _ = fmt.Fprintln(logf, msg)
			}
		} else {
			if err := compileDnsmasqChnroutes(logf, started); err != nil {
				finishDnsmasqChnroutesInstall(started, warpInstallStateFailed, err.Error())
				return
			}
		}
		if !dnsmasq.SupportsChnroutes() {
			finishDnsmasqChnroutesInstall(started, warpInstallStateFailed, "install finished but dnsmasq --help lacks chnroutes-file")
			return
		}
		if srv.setupComplete() {
			st := srv.store.Get()
			cfg := st.DHCP
			if cfg.Interface == "" {
				cfg.Interface = srv.env.DevLAN
			}
			if err := store.NormalizeDHCP(&cfg, srv.env.DevLAN); err == nil {
				// 无论是否启用：Apply 会在未启用时 stop+disable，避免 apt 默认自启残留。
				_ = dnsmasq.Apply(cfg, srv.dnsmasqOpts(st))
			}
		}
		srv.auditLog(r, "dhcp.dnsmasq.install_chnroutes", "")
		finishDnsmasqChnroutesInstall(started, warpInstallStateOK, "patched dnsmasq installed")
	}()
	return nil
}

func installDnsmasqChnroutesPrebuilt(logf *os.File, started string) (bool, string) {
	saveDnsmasqChnroutesInstallStatus(dnsmasqChnroutesInstallStatus{
		State:     warpInstallStateRunning,
		Message:   "installing prebuilt dnsmasq-chnroutes",
		StartedAt: started,
	})
	path, err := dnsmasq.LocatePrebuiltChnroutes()
	if err != nil {
		return false, ""
	}
	_, _ = fmt.Fprintf(logf, "using local prebuilt: %s\n", path)
	if err := dnsmasq.InstallChnroutesBinary(path); err != nil {
		_, _ = fmt.Fprintf(logf, "prebuilt install failed: %v\n", err)
		return false, ""
	}
	return true, "installed from local prebuilt"
}

func installDnsmasqChnroutesFromRelease(logf *os.File, started string) (bool, string) {
	saveDnsmasqChnroutesInstallStatus(dnsmasqChnroutesInstallStatus{
		State:     warpInstallStateRunning,
		Message:   "installing dnsmasq-chnroutes from release bundle",
		StartedAt: started,
	})
	_, _ = fmt.Fprintln(logf, "try release bundle…")
	if err := releasecatalog.InstallDnsmasqChnroutesFromRelease(""); err != nil {
		_, _ = fmt.Fprintf(logf, "release prebuilt: %v\n", err)
		return false, ""
	}
	return true, "installed from release bundle"
}

func compileDnsmasqChnroutes(logf *os.File, started string) error {
	saveDnsmasqChnroutesInstallStatus(dnsmasqChnroutesInstallStatus{
		State:     warpInstallStateRunning,
		Message:   "compiling patched dnsmasq (fallback, may take several minutes)",
		StartedAt: started,
	})
	script, err := resolveChnroutesBuildScript()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(logf, "fallback compile: %s\n", script)
	cmd := exec.Command("bash", script)
	cmd.Stdout = logf
	cmd.Stderr = logf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("build failed: %w", err)
	}
	return nil
}
