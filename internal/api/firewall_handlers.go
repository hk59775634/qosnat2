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
		srv.handleFirewallRulesGet(w, r)
	case http.MethodPost:
		srv.handleFirewallRulesPost(w, r)
	case http.MethodPut:
		srv.handleFirewallRulesPut(w, r)
	case http.MethodDelete:
		srv.handleFirewallRulesDelete(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleFirewallRulesGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	applied := srv.appliedFilterRules(st)
	needSave := false
	if fixed, ok := store.RepairFilterRuleIDs(applied); ok {
		applied = fixed
		needSave = true
	}
	vp := nft.VPNFirewallFromState(st)
	wanDevs := store.CollectWanInputDevices(srv.env.DevWAN, srv.env.DevLAN, st)
	if synced, ok := store.SyncAutoFilterRules(applied, wanDevs, srv.env.AdminPort, nft.AutoInputFromState(st), st.Firewall.WanPortForwards, st.LVS, srv.env.DevLAN, srv.env.DevWAN, nft.HairpinAddrResolver(srv.env.DevLAN, srv.env.DevWAN)); ok {
		applied = synced
		needSave = true
	}
	if needSave {
		_ = srv.store.Update(func(s *store.State) {
			s.Firewall.FilterRules = applied
		})
		if !srv.persistState(w) {
			return
		}
		st = srv.store.Get()
	}
	if st.Firewall.PendingFilterDraft {
		srv.refreshPendingAutoRules()
		st = srv.store.Get()
	}
	rules := srv.workingFilterRules(st)
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
	changes := srv.buildFirewallChangesPayload(st)
	writeJSON(w, http.StatusOK, map[string]any{
		"rules":                  rules,
		"applied_rules":          srv.appliedFilterRules(st),
		"changes":                changes,
		"nft_lines":              firewallNftLines(rules, st.Firewall.Schedules),
		"schedules":              st.Firewall.Schedules,
		"wan_links":              st.Network.WanLinks,
		"dev_lan":                srv.env.DevLAN,
		"dev_wan":                srv.env.DevWAN,
		"admin_port":             srv.env.AdminPort,
		"interfaces":             ifaces,
		"alias_names":            aliasNames,
		"vpn": map[string]any{
			"ocserv_enabled":    vp.OCServEnabled,
			"ocserv_tcp_port":   vp.OCServTCP,
			"ocserv_udp_port":   vp.OCServUDP,
			"wireguard_enabled": wgEnabled,
			"wireguard_port":    wgPrimary,
			"wireguard_ports":   vp.WGPorts,
		},
		"acme_temp_allow_http01":     st.System.AcmeTempAllowHTTP01,
		"acme_temp_allow_http01_ips": st.System.AcmeTempAllowHTTP01IPs,
		"max_sessions_per_ip":        st.Firewall.MaxSessionsPerIP,
		"session_limit_cidrs":        store.CollectSessionLimitCIDRs(st),
		"rendered":               srv.firewallRendered(),
	})
}

func (srv *Server) handleFirewallRulesPost(w http.ResponseWriter, r *http.Request) {
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
	if err := store.ValidateFilterRulePortAliases(body, st.Firewall.Aliases); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := srv.validateFilterRuleExtras(body, st); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	proposed := st
	base := srv.workingFilterRules(st)
	proposed.Firewall.FilterRules = append(store.CloneFilterRules(base), body)
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}
	if queryDryRun(r) {
		writeDryRunOK(w, map[string]any{"rule": body, "nft_line": body.NftRuleLine()})
		return
	}
	srv.ensurePendingFilterDraft()
	st = srv.store.Get()
	pending := append(store.CloneFilterRules(st.Firewall.PendingFilterRules), body)
	pending = srv.syncFilterRulesForState(st, pending)
	srv.setPendingFilterRules(pending)
	if !srv.persistState(w) {
		return
	}
	srv.stageFilterRulesResponse(w, r, "firewall.rule.stage.add", body.ID+" "+body.Action)
}

func (srv *Server) handleFirewallRulesPut(w http.ResponseWriter, r *http.Request) {
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
	st := srv.store.Get()
	var prev store.FilterRule
	found := false
	for _, rule := range srv.workingFilterRules(st) {
		if rule.ID == id {
			prev = rule
			found = true
			break
		}
	}
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
	if err := store.ValidateFilterRuleAliases(body, st.Firewall.Aliases); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := store.ValidateFilterRulePortAliases(body, st.Firewall.Aliases); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := srv.validateFilterRuleExtras(body, st); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	base := srv.workingFilterRules(st)
	newRules := store.CloneFilterRules(base)
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
	srv.ensurePendingFilterDraft()
	st = srv.store.Get()
	pending := store.CloneFilterRules(st.Firewall.PendingFilterRules)
	for i := range pending {
		if pending[i].ID == id {
			pending[i] = body
			break
		}
	}
	pending = srv.syncFilterRulesForState(st, pending)
	srv.setPendingFilterRules(pending)
	if !srv.persistState(w) {
		return
	}
	srv.stageFilterRulesResponse(w, r, "firewall.rule.stage.put", id)
}

func (srv *Server) handleFirewallRulesDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		writeBadRequest(w, "id query required")
		return
	}
	st := srv.store.Get()
	working := srv.workingFilterRules(st)
	var target *store.FilterRule
	for i := range working {
		if working[i].ID == id {
			target = &working[i]
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
	for _, rule := range working {
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
	srv.ensurePendingFilterDraft()
	st = srv.store.Get()
	pending := store.CloneFilterRules(st.Firewall.PendingFilterRules)
	out := pending[:0]
	for _, rule := range pending {
		if rule.ID != id {
			out = append(out, rule)
		}
	}
	pending = srv.syncFilterRulesForState(st, out)
	srv.setPendingFilterRules(pending)
	if !srv.persistState(w) {
		return
	}
	srv.stageFilterRulesResponse(w, r, "firewall.rule.stage.delete", id)
}

func (srv *Server) handleFirewallRulesOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}
	var body struct {
		Order []string `json:"order"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	st := srv.store.Get()
	if len(body.Order) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "rules": srv.workingFilterRules(st)})
		return
	}
	reordered, err := store.ReorderFirewallRules(srv.workingFilterRules(st), body.Order)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	reordered = srv.syncFilterRulesForState(st, reordered)
	proposed := st
	proposed.Firewall.FilterRules = reordered
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}
	srv.ensurePendingFilterDraft()
	st = srv.store.Get()
	pending := srv.syncFilterRulesForState(st, reordered)
	srv.setPendingFilterRules(pending)
	if !srv.persistState(w) {
		return
	}
	srv.stageFilterRulesResponse(w, r, "firewall.rule.stage.order", "reordered")
}

func (srv *Server) firewallRendered() string {
	st := srv.store.Get()
	patched := st
	if st.Firewall.PendingFilterDraft {
		patched.Firewall.FilterRules = st.Firewall.PendingFilterRules
	}
	body, err := nft.Render(srv.nftCfg(), patched)
	if err != nil {
		return ""
	}
	return body
}
