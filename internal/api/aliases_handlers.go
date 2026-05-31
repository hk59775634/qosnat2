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
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeAlias(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		st := srv.store.Get()
		var newAliases []store.AliasSet
		replaced := false
		for _, a := range st.Firewall.Aliases {
			if a.Name == body.Name {
				newAliases = append(newAliases, body)
				replaced = true
				continue
			}
			newAliases = append(newAliases, a)
		}
		if !replaced {
			newAliases = append(newAliases, body)
		}
		proposed := st
		proposed.Firewall.Aliases = newAliases
		if err := srv.checkNftForState(proposed); err != nil {
			writeNftApplyError(w, err)
			return
		}
		backup := cloneAliases(st.Firewall.Aliases)
		srv.setAliases(newAliases)
		if !srv.saveState(w) {
			srv.setAliases(backup)
			return
		}
		if err := srv.reloadNftWithAliasRevert(backup); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "firewall.alias.add", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "name": body.Name})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			writeBadRequest(w, "name required")
			return
		}
		st := srv.store.Get()
		var newAliases []store.AliasSet
		found := false
		for _, a := range st.Firewall.Aliases {
			if a.Name == name {
				found = true
				continue
			}
			newAliases = append(newAliases, a)
		}
		if !found {
			writeNotFound(w, "alias not found")
			return
		}
		if store.AliasReferencedByRules(st.Firewall.FilterRules, name) {
			writeConflict(w, "alias is referenced by firewall rules")
			return
		}
		proposed := st
		proposed.Firewall.Aliases = newAliases
		if err := srv.checkNftForState(proposed); err != nil {
			writeNftApplyError(w, err)
			return
		}
		backup := cloneAliases(st.Firewall.Aliases)
		srv.setAliases(newAliases)
		if !srv.saveState(w) {
			srv.setAliases(backup)
			return
		}
		if err := srv.reloadNftWithAliasRevert(backup); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "firewall.alias.delete", name)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}
