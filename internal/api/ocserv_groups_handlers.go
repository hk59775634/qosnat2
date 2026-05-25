package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleOCServGroups(w http.ResponseWriter, r *http.Request) {
	if store.OCServUsesRadius(srv.store.Get().VPN.OCServ) && r.Method != http.MethodGet {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "RADIUS 模式下组由外部目录管理"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"groups":               st.VPN.OCServ.Groups,
			"config_per_group":     st.VPN.OCServ.ConfigPerGroup,
			"default_group_config": st.VPN.OCServ.DefaultGroupConfig,
			"auto_select_group":    st.VPN.OCServ.AutoSelectGroup,
			"default_select_group": st.VPN.OCServ.DefaultSelectGroup,
		})
	case http.MethodPost, http.MethodPut:
		var body store.OCServGroup
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if err := store.NormalizeOCServGroups(&[]store.OCServGroup{body}); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		body = ([]store.OCServGroup{body})[0]
		found := false
		_ = srv.store.Update(func(s *store.State) {
			for i, g := range s.VPN.OCServ.Groups {
				if g.Name == body.Name {
					found = true
					s.VPN.OCServ.Groups[i] = body
					return
				}
			}
			s.VPN.OCServ.Groups = append(s.VPN.OCServ.Groups, body)
		})
		if r.Method == http.MethodPut && !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
			return
		}
		_ = srv.store.Save()
		st := srv.store.Get().VPN.OCServ
		if err := store.NormalizeOCServ(&st); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := ocserv.WriteGroupConfigs(st); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.group.save", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			var out []store.OCServGroup
			for _, g := range s.VPN.OCServ.Groups {
				if g.Name == name {
					found = true
					continue
				}
				out = append(out, g)
			}
			s.VPN.OCServ.Groups = out
			var users []store.OCServUser
			for _, u := range s.VPN.OCServ.Users {
				if u.Group == name {
					u.Group = ""
				}
				users = append(users, u)
			}
			s.VPN.OCServ.Users = users
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "group not found"})
			return
		}
		_ = srv.store.Save()
		st := srv.store.Get().VPN.OCServ
		_ = ocserv.WriteGroupConfigs(st)
		_ = ocserv.SyncPlainUsers(st)
		srv.auditLog(r, "vpn.ocserv.group.delete", name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleOCServVhosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{"vhosts": st.VPN.OCServ.Vhosts})
	case http.MethodPost, http.MethodPut:
		var body store.OCServVhost
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.Enabled = true
		st := srv.store.Get().VPN.OCServ
		if err := store.NormalizeOCServVhosts(&[]store.OCServVhost{body}, st.AuthMethod); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		body = ([]store.OCServVhost{body})[0]
		found := false
		_ = srv.store.Update(func(s *store.State) {
			for i, v := range s.VPN.OCServ.Vhosts {
				if v.Domain == body.Domain {
					found = true
					s.VPN.OCServ.Vhosts[i] = body
					return
				}
			}
			s.VPN.OCServ.Vhosts = append(s.VPN.OCServ.Vhosts, body)
		})
		if r.Method == http.MethodPut && !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "vhost not found"})
			return
		}
		_ = srv.store.Save()
		st = srv.store.Get().VPN.OCServ
		if err := store.NormalizeOCServ(&st); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := ocserv.WriteConf(st); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.vhost.save", body.Domain)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		domain := strings.TrimSpace(r.URL.Query().Get("domain"))
		if domain == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain required"})
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			var out []store.OCServVhost
			for _, v := range s.VPN.OCServ.Vhosts {
				if strings.EqualFold(v.Domain, domain) {
					found = true
					continue
				}
				out = append(out, v)
			}
			s.VPN.OCServ.Vhosts = out
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "vhost not found"})
			return
		}
		_ = srv.store.Save()
		st := srv.store.Get().VPN.OCServ
		if err := ocserv.WriteConf(st); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.vhost.delete", domain)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
