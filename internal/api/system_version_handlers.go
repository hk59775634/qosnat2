package api

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
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
		writeMethodNotAllowed(w)
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
	resp["switch_task"] = getVersionSwitchStatus()
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleSystemVersionSwitchVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var body struct {
		CurrentPasswd string `json:"current_password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	st := srv.store.Get()
	if !srv.verifyAdmin(st.AdminUser, body.CurrentPasswd) {
		writeForbidden(w, "", "current password incorrect")
		return
	}
	if tok := sessionTokenFromRequest(r); tok != "" {
		srv.versionSwitchGrants.grant(tok)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (srv *Server) versionSwitchAuthorized(r *http.Request, passwd string) (ok bool, viaGrant bool) {
	if tok := sessionTokenFromRequest(r); tok != "" && srv.versionSwitchGrants.consume(tok) {
		return true, true
	}
	if passwd == "" {
		return false, false
	}
	st := srv.store.Get()
	return srv.verifyAdmin(st.AdminUser, passwd), false
}

func (srv *Server) versionSwitchRegrant(r *http.Request) {
	if tok := sessionTokenFromRequest(r); tok != "" {
		srv.versionSwitchGrants.grant(tok)
	}
}

func (srv *Server) handleSystemVersionSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "版本切换需要 root 运行 qosnatd")
		return
	}
	var body struct {
		Tag           string `json:"tag"`
		CurrentPasswd string `json:"current_password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	versionID := releasecatalog.NormalizeID(body.Tag)
	if versionID == "" {
		writeBadRequest(w, "tag required")
		return
	}
	if !releasecatalog.ValidID(versionID) {
		writeBadRequest(w, "invalid version id (expected YYYYMMDDNN)")
		return
	}
	authorized, viaGrant := srv.versionSwitchAuthorized(r, body.CurrentPasswd)
	if !authorized {
		writeForbidden(w, "", "password verification required; confirm in version switch dialog")
		return
	}
	if err := srv.startVersionSwitchAsync(r, versionID); err != nil {
		if viaGrant {
			srv.versionSwitchRegrant(r)
		}
		writeConflictWithExtra(w, err.Error(), map[string]any{"job": getVersionSwitchStatus()})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "版本切换已在后台开始",
		"tag":     versionID,
		"job":     getVersionSwitchStatus(),
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
