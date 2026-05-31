package api

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleSystemStateExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		writeApplyError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="qosnat2-state.json"`)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(b)
	srv.auditLog(r, "system.state.export", srv.store.Path())
}

func (srv *Server) handleSystemStateImport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		CurrentPassword string          `json:"current_password"`
		State           json.RawMessage `json:"state"`
	}
	if err := readJSON(r, &body); err != nil {
		writeAPIError(w, http.StatusBadRequest, "VALID_BAD_JSON", "bad json")
		return
	}
	if len(body.State) == 0 {
		writeAPIError(w, http.StatusBadRequest, "VALID_REQUIRED", "state required")
		return
	}
	st := srv.store.Get()
	if !srv.verifyAdmin(st.AdminUser, body.CurrentPassword) {
		writeAPIError(w, http.StatusForbidden, "AUTH_FORBIDDEN", "current password required to import state")
		return
	}
	var imported store.State
	if err := json.Unmarshal(body.State, &imported); err != nil {
		writeAPIError(w, http.StatusBadRequest, "VALID_BAD_STATE", "invalid state json: "+err.Error())
		return
	}
	if strings.TrimSpace(imported.AdminUser) == "" {
		imported.AdminUser = st.AdminUser
	}
	if imported.AdminPassHash == "" {
		imported.AdminPassHash = st.AdminPassHash
	}
	srv.store.ReplaceState(imported)
	if err := srv.store.Save(); err != nil {
		writeSaveError(w, err)
		return
	}
	warn := srv.tryReloadNft()
	if imported.Nat.Nat64Enabled || imported.Nat.Nptv6Enabled {
		if err := srv.applyNatStack(); err != nil {
			warn = strings.TrimSpace(warn + "; nat stack: " + err.Error())
		}
	}
	srv.auditLog(r, "system.state.import", srv.store.Path())
	resp := map[string]any{"ok": true}
	if warn != "" {
		resp["warning"] = warn
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleSystemStateImportFile accepts raw state.json upload (multipart or raw body) with password query/header.
func (srv *Server) handleSystemStateImportRaw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	pass := r.URL.Query().Get("current_password")
	if pass == "" {
		pass = r.Header.Get("X-Current-Password")
	}
	st := srv.store.Get()
	if !srv.verifyAdmin(st.AdminUser, pass) {
		writeAPIError(w, http.StatusForbidden, "AUTH_FORBIDDEN", "current password required")
		return
	}
	b, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeApplyError(w, err)
		return
	}
	var imported store.State
	if err := json.Unmarshal(b, &imported); err != nil {
		writeAPIError(w, http.StatusBadRequest, "VALID_BAD_STATE", "invalid state json: "+err.Error())
		return
	}
	if strings.TrimSpace(imported.AdminUser) == "" {
		imported.AdminUser = st.AdminUser
	}
	if imported.AdminPassHash == "" {
		imported.AdminPassHash = st.AdminPassHash
	}
	srv.store.ReplaceState(imported)
	if err := srv.store.Save(); err != nil {
		writeSaveError(w, err)
		return
	}
	warn := srv.tryReloadNft()
	if imported.Nat.Nat64Enabled || imported.Nat.Nptv6Enabled {
		if err := srv.applyNatStack(); err != nil {
			warn = strings.TrimSpace(warn + "; nat stack: " + err.Error())
		}
	}
	srv.auditLog(r, "system.state.import", "raw")
	resp := map[string]any{"ok": true}
	if warn != "" {
		resp["warning"] = warn
	}
	writeJSON(w, http.StatusOK, resp)
}
