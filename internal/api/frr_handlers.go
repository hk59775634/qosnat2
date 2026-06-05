package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/frr"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFRR(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		extra, _ := frr.ReadExtra()
		managed, _ := frr.RenderManaged(st.Routes)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":          frr.Status(),
			"route_backend":   route.NormalizeBackend(st.System.RouteBackend),
			"boot_on_startup": st.System.FrrBootOnStartup,
			"root":            os.Getuid() == 0,
			"install_job":     getFrrInstallStatus(),
			"config_files":    frr.EditableConfigKeys(),
			"paths": map[string]string{
				"frr_conf": frr.FRRConfPath,
				"managed":  frr.ManagedRoutes,
				"extra":    frr.ExtraConfig,
				"include":  frr.IncludeSnippet,
				"daemons":  frr.DaemonsPath,
			},
			"extra_config":   extra,
			"managed_config": managed,
		})
	case http.MethodPut:
		var body struct {
			RouteBackend   *string `json:"route_backend"`
			ExtraConfig    *string `json:"extra_config"`
			BootOnStartup  *bool   `json:"boot_on_startup"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if os.Getuid() != 0 {
			writeForbidden(w, "", "FRR 设置需要 root 运行 qosnatd")
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			if body.RouteBackend != nil {
				st.System.RouteBackend = route.NormalizeBackend(*body.RouteBackend)
			}
			if body.BootOnStartup != nil {
				st.System.FrrBootOnStartup = *body.BootOnStartup
			}
		})
		if body.ExtraConfig != nil {
			if err := frr.WriteExtra(*body.ExtraConfig); err != nil {
				writeInternalError(w, err.Error())
				return
			}
		}
		if !srv.persistState(w) {
			return
		}
		st := srv.store.Get()
		if body.BootOnStartup != nil && frr.PackageInstalled() {
			if err := frr.SetBootEnabled(st.System.FrrBootOnStartup); err != nil {
				writeInternalError(w, err.Error())
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":              true,
			"route_backend":   route.NormalizeBackend(st.System.RouteBackend),
			"boot_on_startup": st.System.FrrBootOnStartup,
			"status":          frr.Status(),
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleFRRService(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "需要 root")
		return
	}
	var body struct {
		Action string `json:"action"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	action := strings.TrimSpace(strings.ToLower(body.Action))
	if err := frr.ServiceAction(action); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.auditLog(r, "frr.service."+action, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": frr.Status(),
	})
}

func (srv *Server) handleFRRConfig(w http.ResponseWriter, r *http.Request) {
	which := strings.TrimSpace(r.URL.Query().Get("which"))
	if which == "" {
		which = "frr.conf"
	}
	switch r.Method {
	case http.MethodGet:
		path, content, err := frr.ReadConfigFile(which)
		if err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"which":   which,
			"path":    path,
			"content": content,
		})
	case http.MethodPut:
		if os.Getuid() != 0 {
			writeForbidden(w, "", "需要 root")
			return
		}
		var body struct {
			Which   string `json:"which"`
			Content string `json:"content"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		wkey := strings.TrimSpace(body.Which)
		if wkey == "" {
			wkey = which
		}
		path, err := frr.WriteConfigFile(wkey, body.Content)
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "frr.config.write", wkey)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "path": path})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleFRRApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	if route.NormalizeBackend(st.System.RouteBackend) != route.BackendFRR {
		writeBadRequest(w, "route_backend 不是 frr")
		return
	}
	if !frr.PackageInstalled() {
		writeBadRequest(w, "FRR 未安装")
		return
	}
	res, err := route.ApplyManagedRoutes(st.Routes, route.BackendFRR)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res})
}
