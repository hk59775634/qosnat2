package api

import (
	"net/http"
	"os"

	"github.com/hk59775634/qosnat2/internal/frr"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFRRDynamicRouting(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		if route.NormalizeBackend(st.System.RouteBackend) != route.BackendFRR {
			writeBadRequest(w, "route_backend 不是 frr")
			return
		}
		rendered := frr.RenderDynamic(st.DynamicRouting)
		writeJSON(w, http.StatusOK, map[string]any{
			"dynamic_routing": st.DynamicRouting,
			"rendered_config": rendered,
			"runtime_status":  frr.DynamicStatus(st.DynamicRouting),
			"frr_installed":   frr.PackageInstalled(),
			"frr_active":      frr.ServiceActive(),
		})
	case http.MethodPut:
		if os.Getuid() != 0 {
			writeForbidden(w, "", "动态路由配置需要 root 运行 qosnatd")
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
		var body struct {
			DynamicRouting store.DynamicRoutingState `json:"dynamic_routing"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		dr := body.DynamicRouting
		if err := store.NormalizeDynamicRouting(&dr); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		prev, err := store.CloneState(st)
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			s.DynamicRouting = dr
		})
		if err := srv.store.Save(); err != nil {
			srv.store.ReplaceState(prev)
			writeInternalError(w, err.Error())
			return
		}
		if err := frr.ApplyDynamic(dr); err != nil {
			srv.store.ReplaceState(prev)
			_ = srv.store.Save()
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "frr.dynamic_routing.update", "")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":              true,
			"dynamic_routing": dr,
			"runtime_status":  frr.DynamicStatus(dr),
		})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleFRRDynamicRoutingApply(w http.ResponseWriter, r *http.Request) {
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
	if err := frr.ApplyDynamic(st.DynamicRouting); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":             true,
		"runtime_status": frr.DynamicStatus(st.DynamicRouting),
	})
}

func (srv *Server) handleFRRDynamicRoutingStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	writeJSON(w, http.StatusOK, map[string]any{
		"runtime_status": frr.DynamicStatus(st.DynamicRouting),
		"frr_installed":  frr.PackageInstalled(),
		"frr_active":     frr.ServiceActive(),
	})
}
