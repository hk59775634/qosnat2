package api

import (
	"log"
	"net/http"
	"reflect"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) refreshURLAliasesLocked() []string {
	st := srv.store.Get()
	updated, warns := store.RefreshURLAliases(st.Firewall.Aliases)
	if !aliasesMembersChanged(st.Firewall.Aliases, updated) {
		return warns
	}
	srv.setAliases(updated)
	if err := srv.store.Save(); err != nil {
		log.Printf("save url aliases: %v", err)
	}
	return warns
}

func aliasesMembersChanged(a, b []store.AliasSet) bool {
	if len(a) != len(b) {
		return true
	}
	byA := store.AliasByName(a)
	byB := store.AliasByName(b)
	if len(byA) != len(byB) {
		return true
	}
	for name, aa := range byA {
		bb, ok := byB[name]
		if !ok {
			return true
		}
		if aa.URLFetchedAt != bb.URLFetchedAt || !reflect.DeepEqual(aa.Members, bb.Members) {
			return true
		}
	}
	return false
}

func (srv *Server) handleFirewallAliasesRefresh(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	name := r.URL.Query().Get("name")
	st := srv.store.Get()
	var targets []store.AliasSet
	for _, a := range st.Firewall.Aliases {
		if name != "" && a.Name != name {
			continue
		}
		if name == "" && a.URL == "" {
			continue
		}
		if name != "" || a.URL != "" {
			targets = append(targets, a)
		}
	}
	if name != "" && len(targets) == 0 {
		writeNotFound(w, "alias not found")
		return
	}
	if len(targets) == 0 {
		writeBadRequest(w, "no aliases with url configured")
		return
	}
	backup := cloneAliases(st.Firewall.Aliases)
	updated := append([]store.AliasSet(nil), st.Firewall.Aliases...)
	var errs []string
	for i := range updated {
		if name != "" && updated[i].Name != name {
			continue
		}
		if updated[i].URL == "" {
			continue
		}
		if err := store.RefreshAliasFromURL(&updated[i]); err != nil {
			errs = append(errs, updated[i].Name+": "+err.Error())
		}
	}
	if len(errs) > 0 && name != "" {
		writeBadRequest(w, errs[0])
		return
	}
	proposed := st
	proposed.Firewall.Aliases = updated
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}
	srv.setAliases(updated)
	if !srv.saveState(w) {
		srv.setAliases(backup)
		return
	}
	if err := srv.reloadNftWithAliasRevert(backup); err != nil {
		writeApplyError(w, err)
		return
	}
	srv.auditLog(r, "firewall.alias.refresh", name)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":      true,
		"errors":  errs,
		"aliases": updated,
	})
}
