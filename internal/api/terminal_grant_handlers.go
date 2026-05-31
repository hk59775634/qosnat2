package api

import (
	"net"
	"net/http"
	"os"
	"strings"
)

func (srv *Server) handleTerminalGrant(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
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
		srv.terminalGrants.grant(tok)
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func terminalClientAllowed(r *http.Request) bool {
	raw := strings.TrimSpace(os.Getenv("QOSNAT_TERMINAL_ALLOW_CIDRS"))
	if raw == "" {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		_, network, err := net.ParseCIDR(part)
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}
