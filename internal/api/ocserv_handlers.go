package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleOCServ(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		o := st.VPN.OCServ
		writeJSON(w, http.StatusOK, map[string]any{
			"config":                ocservPublicConfig(o),
			"status":                ocserv.InstallInfo(),
			"install_script":        ocserv.InstallScriptPath(),
			"install_job":           ocserv.GetInstallStatus(),
			"conf_path":             ocserv.ConfPath,
			"radius_secret_set":     strings.TrimSpace(o.Radius.Secret) != "",
			"camouflage_secret_set": strings.TrimSpace(o.Advanced.CamouflageSecret) != "",
		})
	case http.MethodPut:
		var body store.OCServState
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		prev := srv.store.Get().VPN.OCServ
		mergeOCServPasswords(&body, prev)
		mergeOCServRadiusSecret(&body, prev)
		mergeOCServCamouflageSecret(&body, prev)
		if err := store.NormalizeOCServ(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			s.VPN.OCServ = body
		})
		_ = srv.store.Save()
		st := srv.store.Get().VPN.OCServ
		if err := ocserv.SyncPlainUsers(st); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if err := ocserv.WriteGroupConfigs(st); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.save", "")
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleOCServApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	up := st.VPN.OCServ.Enabled
	mode, err := ocserv.Apply(st.VPN.OCServ, up)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "vpn.ocserv.apply", map[bool]string{true: "up", false: "down"}[up])
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "apply_mode": mode})
}

func (srv *Server) handleOCServInstallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, ocserv.GetInstallStatus())
}

func (srv *Server) handleOCServInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if os.Getuid() != 0 {
		writeJSON(w, http.StatusForbidden, map[string]string{
			"error": "安装需要 root 运行 qosnatd（systemd 未降权或使用 sudo 启动服务）",
		})
		return
	}
	script := ocserv.InstallScriptPath()
	if _, err := os.Stat(script); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "install script not found"})
		return
	}
	if err := ocserv.StartInstallAsync(script); err != nil {
		writeJSON(w, http.StatusConflict, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "vpn.ocserv.install.start", script)
	writeJSON(w, http.StatusAccepted, map[string]any{
		"ok":      true,
		"message": "已在后台开始编译安装，请稍候查看下方进度",
		"script":  script,
		"job":     ocserv.GetInstallStatus(),
	})
}

func (srv *Server) handleOCServUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		list := make([]map[string]string, 0, len(st.VPN.OCServ.Users))
		for _, u := range st.VPN.OCServ.Users {
			list = append(list, map[string]string{
				"username": u.Username,
				"comment":  u.Comment,
				"group":    u.Group,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": list})
	case http.MethodPost:
		if store.OCServUsesRadius(srv.store.Get().VPN.OCServ) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "RADIUS 认证模式下请使用外部用户目录，勿添加本地用户"})
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Comment  string `json:"comment"`
			Group    string `json:"group"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if body.Username == "" || len(body.Password) < 4 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username and password (min 4) required"})
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			for i, u := range s.VPN.OCServ.Users {
				if u.Username == body.Username {
					s.VPN.OCServ.Users[i].Password = body.Password
					s.VPN.OCServ.Users[i].Comment = body.Comment
					s.VPN.OCServ.Users[i].Group = body.Group
					return
				}
			}
			s.VPN.OCServ.Users = append(s.VPN.OCServ.Users, store.OCServUser{
				Username: body.Username,
				Password: body.Password,
				Comment:  body.Comment,
				Group:    body.Group,
			})
		})
		_ = srv.store.Save()
		st := srv.store.Get().VPN.OCServ
		if err := ocserv.SyncPlainUsers(st); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.user.add", body.Username)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "synced": true})
	case http.MethodPut:
		if store.OCServUsesRadius(srv.store.Get().VPN.OCServ) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "RADIUS 认证模式下请使用外部用户目录"})
			return
		}
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Comment  string `json:"comment"`
			Group    string `json:"group"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.Username = strings.TrimSpace(body.Username)
		if body.Username == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username required"})
			return
		}
		if body.Password != "" && len(body.Password) < 4 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "password min 4 chars when changing"})
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			for i, u := range s.VPN.OCServ.Users {
				if u.Username != body.Username {
					continue
				}
				found = true
				if body.Password != "" {
					s.VPN.OCServ.Users[i].Password = body.Password
				}
				s.VPN.OCServ.Users[i].Comment = body.Comment
				s.VPN.OCServ.Users[i].Group = body.Group
				return
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		_ = srv.store.Save()
		if err := ocserv.SyncPlainUsers(srv.store.Get().VPN.OCServ); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.user.update", body.Username)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "synced": true})
	case http.MethodDelete:
		name := r.URL.Query().Get("username")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username required"})
			return
		}
		found := false
		_ = srv.store.Update(func(s *store.State) {
			var out []store.OCServUser
			for _, u := range s.VPN.OCServ.Users {
				if u.Username == name {
					found = true
					continue
				}
				out = append(out, u)
			}
			s.VPN.OCServ.Users = out
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
			return
		}
		_ = srv.store.Save()
		if err := ocserv.SyncPlainUsers(srv.store.Get().VPN.OCServ); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "vpn.ocserv.user.delete", name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "synced": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func ocservPublicConfig(o store.OCServState) store.OCServState {
	out := o
	out.Radius.Secret = ""
	out.Advanced.CamouflageSecret = ""
	out.Users = nil
	for _, u := range o.Users {
		out.Users = append(out.Users, store.OCServUser{
			Username: u.Username,
			Comment:  u.Comment,
			Group:    u.Group,
		})
	}
	return out
}

func mergeOCServRadiusSecret(body *store.OCServState, prev store.OCServState) {
	if body.Radius.Secret == "" && prev.Radius.Secret != "" {
		body.Radius.Secret = prev.Radius.Secret
	}
}

func mergeOCServCamouflageSecret(body *store.OCServState, prev store.OCServState) {
	if body.Advanced.CamouflageSecret == "" && prev.Advanced.CamouflageSecret != "" {
		body.Advanced.CamouflageSecret = prev.Advanced.CamouflageSecret
	}
}

func (srv *Server) handleOCServStatusDetail(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := ocserv.OcctlFromState(srv.store.Get().VPN.OCServ)
	st, err := cfg.ShowStatus()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"status":        ocserv.InstallInfo(),
		"detail":        st,
		"use_occtl":     cfg.UseOcctl,
		"socket_file":   cfg.SocketFile,
	})
}

func (srv *Server) handleOCServSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cfg := ocserv.OcctlFromState(srv.store.Get().VPN.OCServ)
	users, err := cfg.ShowUsers()
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions":    users,
		"use_occtl":   cfg.UseOcctl,
		"socket_file": cfg.SocketFile,
	})
}

func (srv *Server) handleOCServSessionsDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		ID       string `json:"id"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	body.Username = strings.TrimSpace(body.Username)
	body.ID = strings.TrimSpace(body.ID)
	if body.Username == "" && body.ID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "username or id required"})
		return
	}
	cfg := ocserv.OcctlFromState(srv.store.Get().VPN.OCServ)
	var err error
	var target string
	if body.Username != "" {
		err = cfg.DisconnectUser(body.Username)
		target = body.Username
	} else {
		err = cfg.DisconnectID(body.ID)
		target = body.ID
	}
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "vpn.ocserv.session.disconnect", target)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func mergeOCServPasswords(body *store.OCServState, prev store.OCServState) {
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