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
		if synced, ok := store.SyncAutoFilterRules(rules, srv.env.DevWAN, srv.env.AdminPort, store.AutoInputVPN{
			OCServEnabled: vp.OCServEnabled,
			OCServTCP:     vp.OCServTCP,
			OCServUDP:     vp.OCServUDP,
			WGPorts:       vp.WGPorts,
		}); ok {
			rules = synced
			needSave = true
		}
		if needSave {
			_ = srv.store.Update(func(s *store.State) {
				s.Firewall.FilterRules = rules
			})
			_ = srv.store.Save()
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
			"rules":      rules,
			"dev_lan":    srv.env.DevLAN,
			"dev_wan":    srv.env.DevWAN,
			"admin_port": srv.env.AdminPort,
			"interfaces": ifaces,
			"alias_names": aliasNames,
			"vpn": map[string]any{
				"ocserv_enabled":     vp.OCServEnabled,
				"ocserv_tcp_port":    vp.OCServTCP,
				"ocserv_udp_port":    vp.OCServUDP,
				"wireguard_enabled":  wgEnabled,
				"wireguard_port":     wgPrimary,
				"wireguard_ports":    vp.WGPorts,
			},
			"acme_temp_allow_http01": st.System.AcmeTempAllowHTTP01,
			"rendered":               srv.firewallRendered(),
		})
	case http.MethodPost:
		var body store.FilterRule
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.System = false
		if err := store.NormalizeFilterRule(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			st.Firewall.FilterRules = append(st.Firewall.FilterRules, body)
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.rule.add", body.ID+" "+body.Action)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": body.ID, "rule": body})
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
			return
		}
		var body store.FilterRule
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
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
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		if !store.FilterRuleMutable(prev) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "system rule cannot be modified"})
			return
		}
		body.System = prev.System
		if err := store.NormalizeFilterRule(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			for i, rule := range st.Firewall.FilterRules {
				if rule.ID == id {
					st.Firewall.FilterRules[i] = body
					break
				}
			}
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.rule.put", id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rule": body})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
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
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		if !store.FilterRuleMutable(*target) {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "system rule cannot be deleted"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.FilterRule
			for _, rule := range st.Firewall.FilterRules {
				if rule.ID != id {
					out = append(out, rule)
				}
			}
			st.Firewall.FilterRules = out
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
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
	if err := readJSON(r, &body); err != nil || len(body.Order) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "order[] required"})
		return
	}
	var reordered []store.FilterRule
	var err error
	_ = srv.store.Update(func(st *store.State) {
		reordered, err = store.ReorderFirewallRules(st.Firewall.FilterRules, body.Order)
		if err != nil {
			return
		}
		st.Firewall.FilterRules = reordered
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	_ = srv.store.Save()
	if err := srv.reloadNft(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "firewall.rule.order", "reordered")
	if reordered == nil {
		reordered = []store.FilterRule{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rules": reordered})
}

func (srv *Server) firewallRendered() string {
	st := srv.store.Get()
	body, err := nft.Render(srv.nftCfg(), st)
	if err != nil {
		return ""
	}
	return body
}
