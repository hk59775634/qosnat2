package api

import (
	"net/http"

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
		writeJSON(w, http.StatusOK, map[string]any{
			"rules":    rules,
			"dev_lan":  srv.env.DevLAN,
			"dev_wan":  srv.env.DevWAN,
			"rendered": srv.firewallRendered(),
		})
	case http.MethodPost:
		var body store.FilterRule
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
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
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": body.ID})
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
		if err := store.NormalizeFilterRule(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			for i, rule := range st.Firewall.FilterRules {
				if rule.ID == id {
					st.Firewall.FilterRules[i] = body
					found = true
					break
				}
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.rule.put", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
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
	body, err := nft.Render(nft.Config{DevLAN: srv.env.DevLAN, DevWAN: srv.env.DevWAN}, st)
	if err != nil {
		return ""
	}
	return body
}
