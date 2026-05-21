package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFirewallAliases(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		aliases := st.Firewall.Aliases
		if aliases == nil {
			aliases = []store.AliasSet{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"aliases": aliases})
	case http.MethodPost:
		var body store.AliasSet
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if err := store.NormalizeAlias(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			for i, a := range st.Firewall.Aliases {
				if a.Name == body.Name {
					st.Firewall.Aliases[i] = body
					return
				}
			}
			st.Firewall.Aliases = append(st.Firewall.Aliases, body)
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.alias.add", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": body.Name})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.AliasSet
			for _, a := range st.Firewall.Aliases {
				if a.Name != name {
					out = append(out, a)
				}
			}
			st.Firewall.Aliases = out
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.alias.delete", name)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
