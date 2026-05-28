package api

import (
	"net/http"
	"os"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/releasecatalog"
)

func (srv *Server) handleOCServVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, ocserv.VersionInfo())
}

func (srv *Server) handleOCServVersionSwitch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "版本切换需要 root 运行 qosnatd",
		})
		return
	}
	var body struct {
		Version       string `json:"version"`
		AdminPassword string `json:"admin_password"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	version := releasecatalog.NormalizeOcservVersion(body.Version)
	if version == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "version required"})
		return
	}
	if !releasecatalog.ValidOcservVersion(version) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid ocserv version (expected official tag e.g. 1.4.2)"})
		return
	}
	st := srv.store.Get()
	if !srv.verifyAdmin(st.AdminUser, body.AdminPassword) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "incorrect admin password"})
		return
	}
	if err := ocserv.SwitchReleaseVersion(version); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "vpn.ocserv.version.switch", version)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"message": "ocserv 已切换版本并重启服务",
		"version": version,
		"status":  ocserv.InstallInfo(),
	})
}
