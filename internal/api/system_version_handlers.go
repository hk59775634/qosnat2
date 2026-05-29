package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/releasecatalog"
)

const (
	qosnatBinPath    = "/usr/local/bin/qosnatd"
	qosnatReleaseTag = "/etc/qosnat2/release-tag"
)

func (srv *Server) handleSystemVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	currentTag := releasecatalog.NormalizeID(readTextFile(qosnatReleaseTag))
	currentVersion := detectQosnatVersion()
	entries, listErr := releasecatalog.ListEntries("qosnat2")
	releases := releasecatalog.ToReleaseMaps(entries)
	resp := map[string]any{
		"binary_path":     qosnatBinPath,
		"current_tag":     currentTag,
		"current_version": currentVersion,
		"root_required":   os.Getuid() == 0,
		"releases":        releases,
		"manifest_url":    releasecatalog.ManifestURL("qosnat2"),
	}
	if listErr != nil {
		resp["list_error"] = listErr.Error()
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleSystemVersionSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "版本切换需要 root 运行 qosnatd"})
		return
	}
	var body struct {
		Tag           string `json:"tag"`
		CurrentPasswd string `json:"current_password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	versionID := releasecatalog.NormalizeID(body.Tag)
	if versionID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "tag required"})
		return
	}
	if !releasecatalog.ValidID(versionID) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid version id (expected YYYYMMDDNN)"})
		return
	}
	st := srv.store.Get()
	if !srv.verifyAdmin(st.AdminUser, body.CurrentPasswd) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password incorrect"})
		return
	}
	if err := releasecatalog.InstallReleaseBinary(versionID, qosnatBinPath); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	_ = os.MkdirAll(filepath.Dir(qosnatReleaseTag), 0755)
	_ = os.WriteFile(qosnatReleaseTag, []byte(versionID+"\n"), 0644)
	srv.auditLog(r, "system.version.switch", versionID)

	cmd := exec.Command("bash", "-lc", "sleep 1; systemctl restart qosnatd.service")
	_ = cmd.Start()

	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "版本切换完成，服务即将重启",
		"tag":     versionID,
	})
}

func detectQosnatVersion() string {
	if bi, ok := debug.ReadBuildInfo(); ok {
		if v := strings.TrimSpace(bi.Main.Version); v != "" && v != "(devel)" {
			return v
		}
		var rev, t string
		for _, s := range bi.Settings {
			if s.Key == "vcs.revision" {
				rev = s.Value
			}
			if s.Key == "vcs.time" {
				t = s.Value
			}
		}
		rev = strings.TrimSpace(rev)
		if rev != "" {
			if len(rev) > 12 {
				rev = rev[:12]
			}
			if ts, err := time.Parse(time.RFC3339, t); err == nil {
				return fmt.Sprintf("devel-%s (%s)", rev, ts.Format("2006-01-02"))
			}
			return "devel-" + rev
		}
	}
	if out, err := exec.Command(qosnatBinPath, "--version").CombinedOutput(); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	return "unknown"
}

func readTextFile(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}
