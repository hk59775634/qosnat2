package api

import (
	"net/http"
	"os"
	"strconv"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFirewallSessionLimit(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleFirewallSessionLimitGet(w, r)
	case http.MethodPut:
		srv.handleFirewallSessionLimitPut(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleFirewallSessionLimitGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	writeJSON(w, http.StatusOK, srv.firewallSessionLimitPayload(st))
}

func (srv *Server) firewallSessionLimitPayload(st store.State) map[string]any {
	return map[string]any{
		"max_sessions_per_ip": st.Firewall.MaxSessionsPerIP,
		"session_limit_cidrs": store.CollectSessionLimitCIDRs(st),
	}
}

func (srv *Server) handleFirewallSessionLimitPut(w http.ResponseWriter, r *http.Request) {
	if os.Getuid() != 0 {
		writeForbidden(w, "", "session limit requires root")
		return
	}
	var body struct {
		MaxSessionsPerIP int `json:"max_sessions_per_ip"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	limit, err := store.NormalizeMaxSessionsPerIP(body.MaxSessionsPerIP)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	_ = srv.store.Update(func(s *store.State) {
		s.Firewall.MaxSessionsPerIP = limit
	})
	if !srv.persistState(w) {
		return
	}
	if err := srv.reloadNft(); err != nil {
		writeNftApplyError(w, err)
		return
	}
	srv.auditLog(r, "firewall.session_limit", strconv.Itoa(limit))
	st := srv.store.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":                  true,
		"max_sessions_per_ip": st.Firewall.MaxSessionsPerIP,
		"session_limit_cidrs": store.CollectSessionLimitCIDRs(st),
	})
}
