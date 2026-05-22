package api

import (
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/audit"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleSystemGeneral(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		tls := srv.tlsStatus()
		writeJSON(w, http.StatusOK, map[string]any{
			"hostname":       st.System.Hostname,
			"admin_user":     st.AdminUser,
			"dev_lan":        srv.env.DevLAN,
			"dev_wan":        srv.env.DevWAN,
			"setup_complete": st.SetupComplete,
			"tls":            tls,
		})
	case http.MethodPut:
		var body struct {
			Hostname        string `json:"hostname"`
			NewPassword     string `json:"new_password"`
			CurrentPassword string `json:"current_password"`
			TLSEnabled      *bool  `json:"tls_enabled"`
			TLSCert         string `json:"tls_cert"`
			TLSKey          string `json:"tls_key"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		st := srv.store.Get()
		tlsTouched := body.TLSEnabled != nil || strings.TrimSpace(body.TLSCert) != "" || strings.TrimSpace(body.TLSKey) != ""
		if tlsTouched {
			if !srv.verifyAdmin(st.AdminUser, body.CurrentPassword) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password required to change HTTPS settings"})
				return
			}
			enabled := st.System.TLSEnabled
			if body.TLSEnabled != nil {
				enabled = *body.TLSEnabled
			}
			if enabled {
				cert := strings.TrimSpace(body.TLSCert)
				key := strings.TrimSpace(body.TLSKey)
				if cert == "" || key == "" {
					if !tlsFileExists(defaultTLSCertPath) || !tlsFileExists(defaultTLSKeyPath) {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": "paste or upload certificate and private key PEM"})
						return
					}
					// 仅开启：使用已有文件
					srv.env.TLSCert = defaultTLSCertPath
					srv.env.TLSKey = defaultTLSKeyPath
					if err := writeRuntimeEnvMerged(srv.env); err != nil {
						writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
						return
					}
					_ = srv.store.Update(func(st *store.State) { st.System.TLSEnabled = true })
					_ = srv.store.Save()
					srv.reloadEnv()
					if err := restartQoSnatd(); err != nil {
						writeJSON(w, http.StatusOK, map[string]any{"ok": true, "warning": err.Error(), "tls": srv.tlsStatus()})
						return
					}
				} else {
					if err := srv.applyTLS(true, cert, key); err != nil {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
					srv.auditLog(r, "system.tls", "enabled")
				}
			} else {
				if err := srv.applyTLS(false, "", ""); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
				srv.auditLog(r, "system.tls", "disabled")
			}
		}
		if body.NewPassword != "" {
			if len(body.NewPassword) < 8 {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "new_password must be at least 8 characters"})
				return
			}
			if !srv.verifyAdmin(st.AdminUser, body.CurrentPassword) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password incorrect"})
				return
			}
			hash, err := hashPassword(body.NewPassword)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			_ = srv.store.Update(func(st *store.State) {
				st.AdminPassHash = string(hash)
			})
			srv.auditLog(r, "system.password", "changed")
		}
		if h := strings.TrimSpace(body.Hostname); h != "" {
			_ = srv.store.Update(func(st *store.State) {
				st.System.Hostname = h
			})
			if os.Getuid() == 0 {
				_ = exec.Command("hostnamectl", "set-hostname", h).Run()
			}
			srv.auditLog(r, "system.hostname", h)
		}
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tls": srv.tlsStatus()})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleSystemAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 100
	list, err := audit.Tail(limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []audit.Entry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"entries": list,
		"path":    audit.Path(),
	})
}
