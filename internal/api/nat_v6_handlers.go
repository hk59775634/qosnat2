package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/jool"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/unbound"
)

func unboundInstalled() bool { return unbound.Installed() }
func joolInstalled() bool    { return jool.Installed() }

func (srv *Server) handleNatSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"nat":    st.Nat,
		"status": srv.nat64Status(st),
	})
}

func (srv *Server) handleNptv6(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"nptv6_enabled": st.Nat.Nptv6Enabled,
			"nptv6_rules":   st.Nat.Nptv6Rules,
		})
	case http.MethodPut:
		var body struct {
			Nptv6Enabled bool              `json:"nptv6_enabled"`
			Nptv6Rules   []store.Nptv6Rule `json:"nptv6_rules"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if body.Nptv6Enabled {
			for _, rule := range body.Nptv6Rules {
				if err := store.ValidateNptv6Rule(rule); err != nil {
					writeBadRequest(w, err.Error())
					return
				}
			}
		}
		if !srv.commitNatStackChange(w, func(st *store.State) {
			st.Nat.Nptv6Enabled = body.Nptv6Enabled
			st.Nat.Nptv6Rules = body.Nptv6Rules
			store.MigrateNptv6RuleIDs(&st.Nat.Nptv6Rules)
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleNat64(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"nat64_enabled": st.Nat.Nat64Enabled,
			"nat64_prefix":  st.Nat.Nat64Prefix,
			"nat64_pool4":   st.Nat.Nat64Pool4,
			"dns64":         st.Nat.DNS64,
			"status":        srv.nat64Status(st),
		})
	case http.MethodPut:
		var body struct {
			Nat64Enabled bool              `json:"nat64_enabled"`
			Nat64Prefix  string            `json:"nat64_prefix"`
			Nat64Pool4   string            `json:"nat64_pool4"`
			DNS64        store.DNS64Config `json:"dns64"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.Nat64Prefix = strings.TrimSpace(body.Nat64Prefix)
		body.Nat64Pool4 = strings.TrimSpace(body.Nat64Pool4)
		if body.Nat64Prefix == "" {
			body.Nat64Prefix = store.DefaultNat64Prefix
		}
		if body.Nat64Pool4 == "" {
			body.Nat64Pool4 = store.DefaultNat64Pool4
		}
		store.EnsureDNS64Defaults(&body.DNS64)
		candidate := store.NatState{
			Nat64Enabled: body.Nat64Enabled,
			Nat64Prefix:  body.Nat64Prefix,
			Nat64Pool4:   body.Nat64Pool4,
			DNS64:        body.DNS64,
		}
		if err := store.ValidateNat64Config(candidate); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if body.Nat64Enabled && body.DNS64.Mode == store.DNS64ModeLocal && !unboundInstalled() {
			writeBadRequest(w, "unbound not installed (apt install unbound)")
			return
		}
		if body.Nat64Enabled && !joolInstalled() {
			writeBadRequest(w, "jool not installed (apt install jool-tools jool-dkms)")
			return
		}
		if !srv.commitNatStackChange(w, func(st *store.State) {
			st.Nat.Nat64Enabled = body.Nat64Enabled
			st.Nat.Nat64Prefix = body.Nat64Prefix
			st.Nat.Nat64Pool4 = body.Nat64Pool4
			st.Nat.DNS64 = body.DNS64
			store.EnsureDNS64Defaults(&st.Nat.DNS64)
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleDNS64(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().Nat.DNS64)
	case http.MethodPut:
		var body store.DNS64Config
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		st := srv.store.Get()
		candidate := st.Nat
		candidate.DNS64 = body
		store.EnsureDNS64Defaults(&candidate.DNS64)
		if err := store.ValidateNat64Config(candidate); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if !srv.commitNatStackChange(w, func(st *store.State) {
			st.Nat.DNS64 = body
			store.EnsureDNS64Defaults(&st.Nat.DNS64)
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}
