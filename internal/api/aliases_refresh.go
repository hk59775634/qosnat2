package api

import (
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/hk59775634/qosnat2/internal/policyroute"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) refreshDynamicAliasesLocked() []string {
	st := srv.store.Get()
	old := cloneAliases(st.Firewall.Aliases)
	updated, warns := store.RefreshDynamicAliases(st.Firewall.Aliases)
	membersChanged := aliasesMembersDiffer(old, updated)
	metaChanged := aliasesMetaDiffer(old, updated)
	if !membersChanged && !metaChanged {
		return warns
	}
	if membersChanged {
		srv.reapplyEgressAfterAliasChange(old, updated)
	} else {
		srv.setAliases(updated)
	}
	if err := srv.store.Save(); err != nil {
		log.Printf("save dynamic aliases: %v", err)
	}
	return warns
}

// refreshURLAliasesLocked 兼容旧名：刷新 URL + FQDN 动态别名。
func (srv *Server) refreshURLAliasesLocked() []string {
	return srv.refreshDynamicAliasesLocked()
}

func aliasesMembersDiffer(a, b []store.AliasSet) bool {
	byA := store.AliasByName(a)
	byB := store.AliasByName(b)
	if len(byA) != len(byB) {
		return true
	}
	for name, aa := range byA {
		bb, ok := byB[name]
		if !ok || !reflect.DeepEqual(aa.Members, bb.Members) {
			return true
		}
	}
	return false
}

func aliasesMetaDiffer(a, b []store.AliasSet) bool {
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
		if aa.URLFetchedAt != bb.URLFetchedAt || aa.ResolvedAt != bb.ResolvedAt {
			return true
		}
	}
	return false
}

func aliasesMembersChanged(a, b []store.AliasSet) bool {
	return aliasesMembersDiffer(a, b) || aliasesMetaDiffer(a, b)
}

// reapplyEgressAfterAliasChange 先按旧 members 删除 ip rule，再用新 members 安装，避免遗留陈旧目的地址规则。
func (srv *Server) reapplyEgressAfterAliasChange(oldAliases, newAliases []store.AliasSet) {
	st := srv.store.Get()
	if len(st.Network.EgressPolicies) == 0 {
		return
	}
	oldBy := store.AliasByName(oldAliases)
	newBy := store.AliasByName(newAliases)
	changed := map[string]struct{}{}
	for name, oa := range oldBy {
		na, ok := newBy[name]
		if !ok || !reflect.DeepEqual(oa.Members, na.Members) {
			changed[name] = struct{}{}
		}
	}
	for name := range newBy {
		if _, ok := oldBy[name]; !ok {
			changed[name] = struct{}{}
		}
	}
	if len(changed) == 0 {
		return
	}
	for _, p := range st.Network.EgressPolicies {
		src := strings.TrimSpace(p.SrcAlias)
		dst := strings.TrimSpace(p.DstAlias)
		_, srcHit := changed[src]
		_, dstHit := changed[dst]
		if (src != "" && srcHit) || (dst != "" && dstHit) {
			policyroute.DeletePolicy(p, st.Network.WanLinks, oldBy)
		}
	}
	// Install with new aliases already in memory only after setAliases; callers must setAliases then applyEgress.
	// Here we temporarily set aliases so Apply sees new members.
	srv.setAliases(newAliases)
	if err := srv.applyEgressPolicyRoutes(); err != nil {
		log.Printf("egress reapply after alias refresh: %v", err)
	}
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
		if name == "" && !store.AliasNeedsDynamicRefresh(a) {
			continue
		}
		targets = append(targets, a)
	}
	if name != "" && len(targets) == 0 {
		writeNotFound(w, "alias not found")
		return
	}
	if len(targets) == 0 {
		writeBadRequest(w, "no aliases with url or fqdn domains configured")
		return
	}
	backup := cloneAliases(st.Firewall.Aliases)
	updated := append([]store.AliasSet(nil), st.Firewall.Aliases...)
	var errs []string
	var warns []string
	for i := range updated {
		if name != "" && updated[i].Name != name {
			continue
		}
		if name == "" && !store.AliasNeedsDynamicRefresh(updated[i]) {
			continue
		}
		if name != "" && !store.AliasNeedsDynamicRefresh(updated[i]) {
			writeBadRequest(w, "alias has no url or fqdn domains")
			return
		}
		warn, err := store.RefreshAliasDynamic(&updated[i])
		if err != nil {
			errs = append(errs, updated[i].Name+": "+err.Error())
			continue
		}
		if warn != "" {
			warns = append(warns, updated[i].Name+": "+warn)
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
	changed := aliasesMembersDiffer(backup, updated)
	if changed {
		srv.reapplyEgressAfterAliasChange(backup, updated)
	} else {
		srv.setAliases(updated)
	}
	if !srv.saveState(w) {
		srv.setAliases(backup)
		_ = srv.applyEgressPolicyRoutes()
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
		"warnings": warns,
		"aliases": updated,
	})
}
