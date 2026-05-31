package api

import (
	"net/http"
	"sort"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFirewallRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		rules := st.Firewall.FilterRules
		if rules == nil {
			rules = []store.FilterRule{}
		}
		needSave := false
		if fixed, ok := store.RepairFilterRuleIDs(rules); ok {
			rules = fixed
			needSave = true
		}
		vp := nft.VPNFirewallFromState(st)
		wanDevs := store.CollectWanInputDevices(srv.env.DevWAN, srv.env.DevLAN, st)
		if synced, ok := store.SyncAutoFilterRules(rules, wanDevs, srv.env.AdminPort, store.AutoInputVPN{
			OCServEnabled: vp.OCServEnabled,
			OCServTCP:     vp.OCServTCP,
			OCServUDP:     vp.OCServUDP,
			WGPorts:       vp.WGPorts,
		}, st.Firewall.WanPortForwards, srv.env.DevLAN); ok {
			rules = synced
			needSave = true
		}
		if needSave {
			_ = srv.store.Update(func(s *store.State) {
				s.Firewall.FilterRules = rules
			})
			if !srv.persistState(w) {
				return
			}
		}
		aliases := st.Firewall.Aliases
		if aliases == nil {
			aliases = []store.AliasSet{}
		}
		aliasNames := make([]string, 0, len(aliases))
		for _, a := range aliases {
			if n := strings.TrimSpace(a.Name); n != "" {
				aliasNames = append(aliasNames, n)
			}
		}
		sort.Strings(aliasNames)
		wgEnabled := len(vp.WGPorts) > 0
		wgPrimary := 0
		if len(vp.WGPorts) > 0 {
			wgPrimary = vp.WGPorts[0]
		}
		var sysDevs []string
		if list, err := netif.ListDetails(); err == nil {
			for _, d := range list {
				sysDevs = append(sysDevs, d.Name)
			}
		}
		ifaces := store.BuildFirewallIfaceList(st, srv.env.DevLAN, srv.env.DevWAN, sysDevs)
		writeJSON(w, http.StatusOK, map[string]any{
			"rules":       rules,
			"nft_lines":   firewallNftLines(rules),
			"dev_lan":     srv.env.DevLAN,
			"dev_wan":     srv.env.DevWAN,
			"admin_port":  srv.env.AdminPort,
			"interfaces":  ifaces,
			"alias_names": aliasNames,
			"vpn": map[string]any{
				"ocserv_enabled":    vp.OCServEnabled,
				"ocserv_tcp_port":   vp.OCServTCP,
				"ocserv_udp_port":   vp.OCServUDP,
				"wireguard_enabled": wgEnabled,
				"wireguard_port":    wgPrimary,
				"wireguard_ports":   vp.WGPorts,
			},
			"acme_temp_allow_http01": st.System.AcmeTempAllowHTTP01,
			"rendered":               srv.firewallRendered(),
		})
	case http.MethodPost:
		var body store.FilterRule
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.System = false
		if err := store.NormalizeFilterRule(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		st := srv.store.Get()
		if err := store.ValidateFilterRuleAliases(body, st.Firewall.Aliases); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		proposed := st
		proposed.Firewall.FilterRules = append(cloneFilterRules(st.Firewall.FilterRules), body)
		if err := srv.checkNftForState(proposed); err != nil {
			writeNftApplyError(w, err)
			return
		}
		if queryDryRun(r) {
			writeDryRunOK(w, map[string]any{"rule": body, "nft_line": body.NftRuleLine()})
			return
		}
		backup := cloneFilterRules(st.Firewall.FilterRules)
		_ = srv.store.Update(func(st *store.State) {
			st.Firewall.FilterRules = append(st.Firewall.FilterRules, body)
		})
		if !srv.saveState(w) {
			srv.setFilterRules(backup)
			return
		}
		if err := srv.reloadFilterWithOptionalIncremental(backup, filterOpAdd, body); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "firewall.rule.add", body.ID+" "+body.Action)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": body.ID, "rule": body})
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id query required")
			return
		}
		var body store.FilterRule
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.ID = id
		found := false
		var prev store.FilterRule
		_ = srv.store.Update(func(st *store.State) {
			for _, rule := range st.Firewall.FilterRules {
				if rule.ID == id {
					prev = rule
					found = true
					return
				}
			}
		})
		if !found {
			writeNotFound(w, "rule not found")
			return
		}
		if !store.FilterRuleMutable(prev) {
			writeForbidden(w, "", "system rule cannot be modified")
			return
		}
		body.System = prev.System
		if err := store.NormalizeFilterRule(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		st := srv.store.Get()
		if err := store.ValidateFilterRuleAliases(body, st.Firewall.Aliases); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		newRules := cloneFilterRules(st.Firewall.FilterRules)
		for i := range newRules {
			if newRules[i].ID == id {
				newRules[i] = body
				break
			}
		}
		proposed := st
		proposed.Firewall.FilterRules = newRules
		if err := srv.checkNftForState(proposed); err != nil {
			writeNftApplyError(w, err)
			return
		}
		if queryDryRun(r) {
			writeDryRunOK(w, map[string]any{"rule": body, "nft_line": body.NftRuleLine()})
			return
		}
		backup := cloneFilterRules(st.Firewall.FilterRules)
		_ = srv.store.Update(func(st *store.State) {
			for i, rule := range st.Firewall.FilterRules {
				if rule.ID == id {
					st.Firewall.FilterRules[i] = body
					break
				}
			}
		})
		if !srv.saveState(w) {
			srv.setFilterRules(backup)
			return
		}
		if err := srv.reloadFilterWithOptionalIncremental(backup, filterOpReplace, body); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "firewall.rule.put", id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rule": body})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id query required")
			return
		}
		var target *store.FilterRule
		st := srv.store.Get()
		for i := range st.Firewall.FilterRules {
			if st.Firewall.FilterRules[i].ID == id {
				target = &st.Firewall.FilterRules[i]
				break
			}
		}
		if target == nil {
			writeNotFound(w, "rule not found")
			return
		}
		if !store.FilterRuleMutable(*target) {
			writeForbidden(w, "", "system rule cannot be deleted")
			return
		}
		var newRules []store.FilterRule
		for _, rule := range st.Firewall.FilterRules {
			if rule.ID != id {
				newRules = append(newRules, rule)
			}
		}
		proposed := st
		proposed.Firewall.FilterRules = newRules
		if err := srv.checkNftForState(proposed); err != nil {
			writeNftApplyError(w, err)
			return
		}
		backup := cloneFilterRules(st.Firewall.FilterRules)
		srv.setFilterRules(newRules)
		if !srv.saveState(w) {
			srv.setFilterRules(backup)
			return
		}
		if err := srv.reloadFilterWithOptionalIncremental(backup, filterOpDelete, *target); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "firewall.rule.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleFirewallRulesOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Order []string `json:"order"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	if len(body.Order) == 0 {
		st := srv.store.Get()
		for _, rule := range st.Firewall.FilterRules {
			if store.FilterRuleMutable(rule) {
				writeBadRequest(w, "order[] required")
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rules": st.Firewall.FilterRules})
		return
	}
	var reordered []store.FilterRule
	var err error
	st := srv.store.Get()
	reordered, err = store.ReorderFirewallRules(st.Firewall.FilterRules, body.Order)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	vp := nft.VPNFirewallFromState(st)
	wanDevs := store.CollectWanInputDevices(srv.env.DevWAN, srv.env.DevLAN, st)
	synced, _ := store.SyncAutoFilterRules(reordered, wanDevs, srv.env.AdminPort, store.AutoInputVPN{
		OCServEnabled: vp.OCServEnabled,
		OCServTCP:     vp.OCServTCP,
		OCServUDP:     vp.OCServUDP,
		WGPorts:       vp.WGPorts,
	}, st.Firewall.WanPortForwards, srv.env.DevLAN)
	proposed := st
	proposed.Firewall.FilterRules = synced
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}
	backup := cloneFilterRules(st.Firewall.FilterRules)
	srv.setFilterRules(synced)
	if !srv.saveState(w) {
		srv.setFilterRules(backup)
		return
	}
	if err := srv.reloadNftWithFilterRevert(backup); err != nil {
		writeApplyError(w, err)
		return
	}
	srv.auditLog(r, "firewall.rule.order", "reordered")
	if synced == nil {
		synced = []store.FilterRule{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rules": synced})
}

func (srv *Server) firewallRendered() string {
	st := srv.store.Get()
	body, err := nft.Render(srv.nftCfg(), st)
	if err != nil {
		return ""
	}
	return body
}
