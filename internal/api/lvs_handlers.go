package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/lvs"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleLVS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		cfg := st.LVS
		_ = store.NormalizeLVS(&cfg, srv.env.DevWAN)
		writeJSON(w, http.StatusOK, map[string]any{
			"config":   cfg,
			"status":   lvs.ShowStatus(),
			"dev_wan":  srv.env.DevWAN,
			"dev_lan":  srv.env.DevLAN,
			"root":     os.Getuid() == 0,
			"ipvsadm":  lvs.ShowStatus().Installed,
		})
	case http.MethodPut:
		var body store.LVSState
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeLVS(&body, srv.env.DevWAN); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		st := srv.store.Get()
		for _, vs := range body.VirtualServers {
			if store.LVSVSConflictsForward(vs, st.Firewall.WanPortForwards) {
				writeBadRequest(w, "virtual server conflicts with existing WAN port forward on same vip:port")
				return
			}
		}
		_ = srv.store.Update(func(st *store.State) {
			st.LVS = body
		})
		if !srv.persistState(w) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleLVSApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "应用 LVS 需要 root 运行 qosnatd")
		return
	}
	st := srv.store.Get()
	cfg := st.LVS
	if err := store.NormalizeLVS(&cfg, srv.env.DevWAN); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := srv.applyLVS(cfg); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.auditLog(r, "lvs.apply", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": lvs.ShowStatus(),
	})
}

func (srv *Server) handleLVSInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "安装 ipvsadm 需要 root")
		return
	}
	if err := lvs.Install(); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.auditLog(r, "lvs.install", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": lvs.ShowStatus(),
	})
}

func (srv *Server) handleLVSVirtualServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		var vs store.LVSVirtualServer
		if err := readJSON(r, &vs); err != nil {
			writeBadJSON(w)
			return
		}
		st := srv.store.Get()
		probe := store.LVSState{
			Enabled:        true,
			Mode:           st.LVS.Mode,
			VirtualServers: []store.LVSVirtualServer{vs},
		}
		if err := store.NormalizeLVS(&probe, srv.env.DevWAN); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		vs = probe.VirtualServers[0]
		if store.LVSVSConflictsForward(vs, st.Firewall.WanPortForwards) {
			writeBadRequest(w, "conflicts with WAN port forward on same vip:port")
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.LVS.VirtualServers = append(st.LVS.VirtualServers, vs)
			st.LVS.Enabled = true
		})
		if !srv.persistState(w) {
			return
		}
		if err := srv.applyLVSFromStore(); err != nil {
			writeApplyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, vs)
	case http.MethodDelete:
		id := strings.TrimSpace(r.URL.Query().Get("id"))
		if id == "" {
			writeBadRequest(w, "id query required")
			return
		}
		st := srv.store.Get()
		found := false
		var kept []store.LVSVirtualServer
		for _, vs := range st.LVS.VirtualServers {
			if vs.ID == id {
				found = true
				continue
			}
			kept = append(kept, vs)
		}
		if !found {
			writeNotFound(w, "not found")
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.LVS.VirtualServers = kept
			if len(kept) == 0 {
				st.LVS.Enabled = false
			}
		})
		if !srv.persistState(w) {
			return
		}
		if err := srv.applyLVSFromStore(); err != nil {
			writeApplyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) applyLVSFromStore() error {
	if os.Getuid() != 0 {
		return nil
	}
	st := srv.store.Get()
	return srv.applyLVS(st.LVS)
}

func (srv *Server) applyLVS(cfg store.LVSState) error {
	return lvs.Apply(lvs.Config{DevWAN: srv.env.DevWAN, State: cfg})
}
