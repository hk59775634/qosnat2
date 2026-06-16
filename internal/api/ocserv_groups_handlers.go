package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleOCServGroups(w http.ResponseWriter, r *http.Request) {
	if store.OCServRadiusUsesGroupconfig(srv.store.Get().VPN.OCServ) && r.Method != http.MethodGet {
		writeBadRequest(w, "RADIUS groupconfig 已启用，组配置由外部目录管理")
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
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeOCServGroups(&[]store.OCServGroup{body}); err != nil {
			writeBadRequest(w, err.Error())
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
			writeNotFound(w, "group not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		st := srv.store.Get().VPN.OCServ
		if err := store.NormalizeOCServ(&st); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if err := ocserv.WriteGroupConfigs(st); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "vpn.ocserv.group.save", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			writeBadRequest(w, "name required")
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
			writeNotFound(w, "group not found")
			return
		}
		if err := srv.store.Save(); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		st := srv.store.Get().VPN.OCServ
		if err := ocserv.WriteGroupConfigs(st); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		if err := ocserv.SyncPlainUsers(st); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "vpn.ocserv.group.delete", name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleOCServVhosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"vhosts": ocservPublicVhosts(st.VPN.OCServ.Vhosts, st.VPN.OCServ, st.Certificates),
		})
	case http.MethodPost, http.MethodPut:
		var body store.OCServVhost
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		prevFull := deepCopyOCServState(srv.store.Get().VPN.OCServ)
		st := srv.store.Get().VPN.OCServ
		if r.Method == http.MethodPost {
			body = store.VhostFromGlobal(st, body.Domain, body.Comment, body.AuthMethod)
		}
		prev := findOCServVhost(st.Vhosts, body.Domain)
		mergeOCServVhostSecrets(&body, prev)
		if body.Users == nil {
			body.Users = prev.Users
		}
		mergeOCServVhostUserPasswords(&body, prev)
		if body.Radius != nil {
			tmp := store.OCServState{AuthMethod: store.OCServAuthRadius, Radius: *body.Radius}
			if err := ocserv.NormalizeRadiusConfig(&tmp); err != nil {
				writeBadRequest(w, err.Error())
				return
			}
			body.Radius = &tmp.Radius
		}
		if err := store.NormalizeOCServVhosts(&[]store.OCServVhost{body}, st.AuthMethod); err != nil {
			writeBadRequest(w, err.Error())
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
			if r.Method != http.MethodPut {
				s.VPN.OCServ.Vhosts = append(s.VPN.OCServ.Vhosts, body)
			}
		})
		if r.Method == http.MethodPut && !found {
			writeNotFound(w, "vhost not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		st = srv.store.Get().VPN.OCServ
		if err := store.NormalizeOCServ(&st); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if err := ocserv.WriteVhostRadcliConfig(body); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		if ocserv.VhostCanManagePlainUsers(body, st.AuthMethod) {
			if err := ocserv.SyncUsersToPath(strings.TrimSpace(body.PlainPasswdPath), body.Users); err != nil {
				writeInternalError(w, err.Error())
				return
			}
		}
		full := srv.store.Get()
		if err := ocserv.WriteConf(st, full.Certificates); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.updateOcservRestartHints(prevFull, srv.store.Get().VPN.OCServ)
		srv.auditLog(r, "vpn.ocserv.vhost.save", body.Domain)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	case http.MethodDelete:
		domain := strings.TrimSpace(r.URL.Query().Get("domain"))
		if domain == "" {
			writeBadRequest(w, "domain required")
			return
		}
		prevFull := deepCopyOCServState(srv.store.Get().VPN.OCServ)
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
			writeNotFound(w, "vhost not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		full := srv.store.Get()
		if err := ocserv.WriteConf(full.VPN.OCServ, full.Certificates); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.updateOcservRestartHints(prevFull, srv.store.Get().VPN.OCServ)
		srv.auditLog(r, "vpn.ocserv.vhost.delete", domain)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

type ocservVhostPublic struct {
	store.OCServVhost
	RadiusSecretSet     bool                  `json:"radius_secret_set"`
	CamouflageSecretSet bool                  `json:"camouflage_secret_set"`
	Connection          ocserv.ConnectionInfo `json:"connection,omitempty"`
}

func ocservPublicVhosts(vhosts []store.OCServVhost, global store.OCServState, managed []store.ManagedCertificate) []ocservVhostPublic {
	out := make([]ocservVhostPublic, 0, len(vhosts))
	for _, v := range vhosts {
		p := ocservVhostPublic{OCServVhost: v}
		if v.Radius != nil {
			p.RadiusSecretSet = strings.TrimSpace(v.Radius.Secret) != ""
			r := *v.Radius
			r.Secret = ""
			p.Radius = &r
		}
		p.CamouflageSecretSet = strings.TrimSpace(v.CamouflageSecret) != ""
		p.CamouflageSecret = ""
		p.Users = ocservPublicVhostUsers(v.Users)
		p.Connection = ocserv.BuildVhostConnectionInfo(v, global, managed)
		out = append(out, p)
	}
	return out
}

func findOCServVhost(vhosts []store.OCServVhost, domain string) store.OCServVhost {
	for _, v := range vhosts {
		if strings.EqualFold(v.Domain, domain) {
			return v
		}
	}
	return store.OCServVhost{}
}

func mergeOCServVhostSecrets(body *store.OCServVhost, prev store.OCServVhost) {
	if body.Radius != nil && strings.TrimSpace(body.Radius.Secret) == "" && prev.Radius != nil && strings.TrimSpace(prev.Radius.Secret) != "" {
		body.Radius.Secret = prev.Radius.Secret
	}
	if strings.TrimSpace(body.CamouflageSecret) == "" && strings.TrimSpace(prev.CamouflageSecret) != "" {
		body.CamouflageSecret = prev.CamouflageSecret
	}
}

func mergeAllOCServVhostSecrets(body *store.OCServState, prev store.OCServState) {
	for i := range body.Vhosts {
		if p := findOCServVhost(prev.Vhosts, body.Vhosts[i].Domain); p.Domain != "" {
			mergeOCServVhostSecrets(&body.Vhosts[i], p)
		}
	}
}

func mergeAllOCServVhostUserPasswords(body *store.OCServState, prev store.OCServState) {
	for i := range body.Vhosts {
		if p := findOCServVhost(prev.Vhosts, body.Vhosts[i].Domain); p.Domain != "" {
			mergeOCServVhostUserPasswords(&body.Vhosts[i], p)
		}
	}
}

func mergeOCServVhostUserPasswords(body *store.OCServVhost, prev store.OCServVhost) {
	prevPW := map[string]string{}
	for _, u := range prev.Users {
		if u.Password != "" {
			prevPW[u.Username] = u.Password
		}
	}
	for i := range body.Users {
		if body.Users[i].Password == "" {
			if p, ok := prevPW[body.Users[i].Username]; ok {
				body.Users[i].Password = p
			}
		}
	}
}

func (srv *Server) handleOCServVhostUsers(w http.ResponseWriter, r *http.Request) {
	domain := strings.TrimSpace(r.URL.Query().Get("domain"))
	if domain == "" {
		writeBadRequest(w, "domain required")
		return
	}
	st := srv.store.Get().VPN.OCServ
	v, ok := findOCServVhostPtr(&st, domain)
	if !ok {
		writeNotFound(w, "vhost not found")
		return
	}
	if !ocserv.VhostCanManagePlainUsers(*v, st.AuthMethod) {
		writeBadRequest(w, "vhost plain passwd file required for local user management")
		return
	}
	passwdPath := strings.TrimSpace(v.PlainPasswdPath)

	switch r.Method {
	case http.MethodGet:
		list := make([]map[string]string, 0, len(v.Users))
		for _, u := range v.Users {
			list = append(list, map[string]string{
				"username": u.Username,
				"comment":  u.Comment,
				"group":    u.Group,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"users":       list,
			"passwd_path": passwdPath,
		})
	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Comment  string `json:"comment"`
			Group    string `json:"group"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if body.Username == "" || len(body.Password) < 4 {
			writeBadRequest(w, "username and password (min 4) required")
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			i, ok := findOCServVhostIndexOnly(s.VPN.OCServ.Vhosts, domain)
			if !ok {
				return
			}
			for j, u := range s.VPN.OCServ.Vhosts[i].Users {
				if u.Username == body.Username {
					s.VPN.OCServ.Vhosts[i].Users[j].Password = body.Password
					s.VPN.OCServ.Vhosts[i].Users[j].Comment = body.Comment
					s.VPN.OCServ.Vhosts[i].Users[j].Group = body.Group
					return
				}
			}
			s.VPN.OCServ.Vhosts[i].Users = append(s.VPN.OCServ.Vhosts[i].Users, store.OCServUser{
				Username: body.Username,
				Password: body.Password,
				Comment:  body.Comment,
				Group:    body.Group,
			})
		})
		if !srv.persistState(w) {
			return
		}
		stAfter := srv.store.Get().VPN.OCServ
		vp, _ := findOCServVhostPtr(&stAfter, domain)
		if err := ocserv.SyncUsersToPath(passwdPath, vp.Users); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "vpn.ocserv.vhost.user.add", domain+"/"+body.Username)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "synced": true})
	case http.MethodPut:
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Comment  string `json:"comment"`
			Group    string `json:"group"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if body.Username == "" {
			writeBadRequest(w, "username required")
			return
		}
		if body.Password != "" && len(body.Password) < 4 {
			writeBadRequest(w, "password min 4 chars when changing")
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			i, ok := findOCServVhostIndexOnly(s.VPN.OCServ.Vhosts, domain)
			if !ok {
				return
			}
			for j, u := range s.VPN.OCServ.Vhosts[i].Users {
				if u.Username != body.Username {
					continue
				}
				found = true
				if body.Password != "" {
					s.VPN.OCServ.Vhosts[i].Users[j].Password = body.Password
				}
				s.VPN.OCServ.Vhosts[i].Users[j].Comment = body.Comment
				s.VPN.OCServ.Vhosts[i].Users[j].Group = body.Group
				return
			}
		})
		if !found {
			writeNotFound(w, "user not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		stAfter := srv.store.Get().VPN.OCServ
		vp, _ := findOCServVhostPtr(&stAfter, domain)
		if err := ocserv.SyncUsersToPath(passwdPath, vp.Users); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "vpn.ocserv.vhost.user.update", domain+"/"+body.Username)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "synced": true})
	case http.MethodDelete:
		name := strings.TrimSpace(r.URL.Query().Get("username"))
		if name == "" {
			writeBadRequest(w, "username required")
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			i, ok := findOCServVhostIndexOnly(s.VPN.OCServ.Vhosts, domain)
			if !ok {
				return
			}
			var out []store.OCServUser
			for _, u := range s.VPN.OCServ.Vhosts[i].Users {
				if u.Username == name {
					found = true
					continue
				}
				out = append(out, u)
			}
			s.VPN.OCServ.Vhosts[i].Users = out
		})
		if !found {
			writeNotFound(w, "user not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		stAfter := srv.store.Get().VPN.OCServ
		vp, _ := findOCServVhostPtr(&stAfter, domain)
		if err := ocserv.SyncUsersToPath(passwdPath, vp.Users); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "vpn.ocserv.vhost.user.delete", domain+"/"+name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "synced": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func findOCServVhostPtr(st *store.OCServState, domain string) (*store.OCServVhost, bool) {
	for i := range st.Vhosts {
		if strings.EqualFold(st.Vhosts[i].Domain, domain) {
			return &st.Vhosts[i], true
		}
	}
	return nil, false
}

func ocservPublicVhostUsers(users []store.OCServUser) []store.OCServUser {
	out := make([]store.OCServUser, 0, len(users))
	for _, u := range users {
		out = append(out, store.OCServUser{
			Username: u.Username,
			Comment:  u.Comment,
			Group:    u.Group,
		})
	}
	return out
}

func findOCServVhostIndexOnly(vhosts []store.OCServVhost, domain string) (int, bool) {
	for i := range vhosts {
		if strings.EqualFold(vhosts[i].Domain, domain) {
			return i, true
		}
	}
	return -1, false
}
