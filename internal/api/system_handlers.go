package api

import (
	"log"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/acme"
	"github.com/hk59775634/qosnat2/internal/audit"
	"github.com/hk59775634/qosnat2/internal/netutil"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleSystemGeneral(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.ensureSystemTLSLinkedToLibrary()
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"hostname":       st.System.Hostname,
			"display_name": st.System.DisplayName,
			"admin_user":     st.AdminUser,
			"admin_port":     srv.env.AdminPort,
			"dev_lan":        srv.env.DevLAN,
			"dev_wan":        srv.env.DevWAN,
			"setup_complete": st.SetupComplete,
			"diagnostics_terminal_enabled": st.System.DiagnosticsTerminalEnabled,
			"tls":            srv.tlsStatusWithAcme(),
		})
	case http.MethodPut:
		var body struct {
			Hostname         string `json:"hostname"`
			DisplayName      *string `json:"display_name"`
			AdminPort        string `json:"admin_port"`
			NewPassword      string `json:"new_password"`
			CurrentPassword  string `json:"current_password"`
			TLSEnabled       *bool  `json:"tls_enabled"`
			TLSCert          string `json:"tls_cert"`
			TLSKey           string `json:"tls_key"`
			TLSDomain        string `json:"tls_domain"`
			TLSAcmeEnabled   *bool  `json:"tls_acme_enabled"`
			TLSAcmeEmail     string `json:"tls_acme_email"`
			TLSAcmeStaging   *bool  `json:"tls_acme_staging"`
			TLSAcmeRenewDays *int   `json:"tls_acme_renew_days"`
			TLSManagedCertID *string `json:"tls_managed_cert_id"`
			DiagnosticsTerminalEnabled *bool `json:"diagnostics_terminal_enabled"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		st := srv.store.Get()
		portWarn := ""
		acmeTouched := body.TLSDomain != "" || body.TLSAcmeEnabled != nil || body.TLSAcmeEmail != "" ||
			body.TLSAcmeStaging != nil || body.TLSAcmeRenewDays != nil
		managedCertTouched := body.TLSManagedCertID != nil
		tlsModeTouched := body.TLSEnabled != nil || strings.TrimSpace(body.TLSCert) != "" ||
			strings.TrimSpace(body.TLSKey) != "" || managedCertTouched
		if acmeTouched || tlsModeTouched {
			if !srv.verifyAdmin(st.AdminUser, body.CurrentPassword) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password required to change HTTPS settings"})
				return
			}
		}
		if acmeTouched {
			if d := strings.TrimSpace(body.TLSDomain); d != "" {
				if _, err := acme.NormalizeDomain(d); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
					return
				}
			}
			_ = srv.store.Update(func(s *store.State) {
				if body.TLSDomain != "" {
					s.System.TLSDomain = strings.TrimSpace(body.TLSDomain)
				}
				if body.TLSAcmeEnabled != nil {
					s.System.TLSAcmeEnabled = *body.TLSAcmeEnabled
				}
				if body.TLSAcmeEmail != "" {
					s.System.TLSAcmeEmail = strings.TrimSpace(body.TLSAcmeEmail)
				}
				if body.TLSAcmeStaging != nil {
					s.System.TLSAcmeStaging = *body.TLSAcmeStaging
				}
				if body.TLSAcmeRenewDays != nil && *body.TLSAcmeRenewDays > 0 {
					s.System.TLSAcmeRenewDays = *body.TLSAcmeRenewDays
				}
			})
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		}
		if acmeTouched && !tlsModeTouched {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tls": srv.tlsStatusWithAcme()})
			return
		}
		if tlsModeTouched {
			enabled := st.System.TLSEnabled
			if body.TLSEnabled != nil {
				enabled = *body.TLSEnabled
			}
			if managedCertTouched {
				certID := strings.TrimSpace(*body.TLSManagedCertID)
				if !enabled {
					if _, applyErr := srv.applyTLS(false, "", ""); applyErr != nil {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": applyErr.Error()})
						return
					}
					_ = srv.store.Update(func(s *store.State) {
						s.System.TLSManagedCertID = ""
						s.System.TLSEnabled = false
					})
					if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
					writeJSON(w, http.StatusOK, map[string]any{
						"ok":      true,
						"warning": "已切换为 HTTP 监听（未重启 qosnatd）。请使用 http:// 访问。",
						"tls":     srv.tlsStatusWithAcme(),
					})
					return
				}
				if certID != "" {
					if _, applyErr := srv.applyTLSFromManagedCertID(certID); applyErr != nil {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": applyErr.Error()})
						return
					}
					srv.auditLog(r, "system.tls", "managed_cert:"+certID)
					writeJSON(w, http.StatusOK, map[string]any{
						"ok":      true,
						"warning": "已启用 HTTPS（证书库），监听已热切换。请使用 https:// 访问。",
						"tls":     srv.tlsStatusWithAcme(),
					})
					return
				}
				_ = srv.store.Update(func(s *store.State) { s.System.TLSManagedCertID = "" })
				if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			}
			useAcme := st.System.TLSAcmeEnabled
			if body.TLSAcmeEnabled != nil {
				useAcme = *body.TLSAcmeEnabled
			}
			if enabled && useAcme {
				if _, err := acme.NormalizeDomain(srv.store.Get().System.TLSDomain); err != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "启用 ACME 需填写有效域名"})
					return
				}
				if strings.TrimSpace(srv.store.Get().System.TLSAcmeEmail) == "" {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "启用 ACME 需填写邮箱"})
					return
				}
				// ACME 模式：不在此粘贴证书，由「申请证书」触发
				_ = srv.store.Update(func(s *store.State) { s.System.TLSEnabled = true })
				if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
				writeJSON(w, http.StatusOK, map[string]any{
					"ok":      true,
					"warning": "已保存 ACME 设置，请点击「申请证书」获取 Let's Encrypt 证书（需 80 端口公网可达）",
					"tls":     srv.tlsStatusWithAcme(),
				})
				return
			}
			if enabled {
				cert := strings.TrimSpace(body.TLSCert)
				key := strings.TrimSpace(body.TLSKey)
				_ = srv.store.Update(func(s *store.State) { s.System.TLSAcmeEnabled = false })
				if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
				if _, applyErr := srv.applyTLS(true, cert, key); applyErr != nil {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": applyErr.Error()})
					return
				}
				if cert != "" && key != "" {
					if _, err := srv.upsertSystemTLSManagedCert(cert, key, false, "", "", false); err != nil {
						writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
						return
					}
				}
				srv.auditLog(r, "system.tls", "enabled")
				writeJSON(w, http.StatusOK, map[string]any{
					"ok":      true,
					"warning": "已启用 HTTPS，监听已热切换（未重启 qosnatd）。请使用 https:// 访问。",
					"tls":     srv.tlsStatusWithAcme(),
				})
				return
			}
			if _, applyErr := srv.applyTLS(false, "", ""); applyErr != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": applyErr.Error()})
				return
			}
			srv.auditLog(r, "system.tls", "disabled")
			writeJSON(w, http.StatusOK, map[string]any{
				"ok":      true,
				"warning": "已关闭 HTTPS，监听已切回 HTTP（未重启 qosnatd）。请使用 http:// 访问。",
				"tls":     srv.tlsStatusWithAcme(),
			})
			return
		}
		if p := strings.TrimSpace(body.AdminPort); p != "" && p != srv.env.AdminPort {
			if !srv.verifyAdmin(st.AdminUser, body.CurrentPassword) {
				writeJSON(w, http.StatusForbidden, map[string]string{"error": "current password required to change admin port"})
				return
			}
			validPort, err := netutil.ValidateListenPort(p)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			srv.env.AdminPort = validPort
			if err := writeRuntimeEnvMerged(srv.env); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if err := srv.reloadHTTPListener(); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			if srv.setupComplete() {
				if err := srv.reloadNft(); err != nil {
					writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
					return
				}
			}
			portWarn = "管理端口已更新，请使用新端口访问 Web UI。"
			srv.auditLog(r, "system.admin_port", validPort)
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
		if body.DisplayName != nil {
			dn := strings.TrimSpace(*body.DisplayName)
			if strings.ContainsAny(dn, "\n\r\x00") {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "display_name invalid"})
				return
			}
			_ = srv.store.Update(func(st *store.State) {
				st.System.DisplayName = dn
			})
			srv.auditLog(r, "system.display_name", store.EffectiveDisplayName(dn))
		}
		if body.DiagnosticsTerminalEnabled != nil {
			_ = srv.store.Update(func(st *store.State) {
				st.System.DiagnosticsTerminalEnabled = *body.DiagnosticsTerminalEnabled
			})
			srv.auditLog(r, "system.diagnostics_terminal", fmt.Sprintf("enabled=%v", *body.DiagnosticsTerminalEnabled))
		}
		if err := srv.store.Save(); err != nil {
			writeSaveError(w, err)
			return
		}
		resp := map[string]any{"ok": true, "tls": srv.tlsStatusWithAcme(), "admin_port": srv.env.AdminPort}
		if portWarn != "" {
			resp["warning"] = portWarn
		}
		writeJSON(w, http.StatusOK, resp)
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
