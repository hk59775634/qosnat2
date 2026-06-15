package api

import (
	"fmt"
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
		var warnings []string
		if store.LVSRole(&cfg) == store.LVSRoleDirector && st.VPN.OCServ.Enabled {
			for _, vs := range cfg.VirtualServers {
				if vs.Service == "ocserv" && store.LVSOCServConflictsLocal(vs, st.VPN.OCServ) {
					warnings = append(warnings, "local ocserv enabled on same port as LVS ocserv cluster; disable local ocserv or use a different port")
					break
				}
			}
		}
		if store.LVSRole(&cfg) == store.LVSRoleRS && cfg.Enabled && !st.VPN.OCServ.Enabled {
			warnings = append(warnings, "rs mode: enable local service (e.g. ocserv) listening on VIP:port")
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"config":       cfg,
			"status":       lvs.ShowStatus(),
			"rs_status":    lvs.ShowRSStatus(cfg),
			"dev_wan":      srv.env.DevWAN,
			"dev_lan":      srv.env.DevLAN,
			"root":         os.Getuid() == 0,
			"ipvsadm":      lvs.ShowStatus().Installed,
			"ocserv_hint":  store.LVSOCServClusterHintFromState(st),
			"warnings":     warnings,
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
	if err := srv.applyLVSFromStore(); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.auditLog(r, "lvs.apply", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"status":    lvs.ShowStatus(),
		"rs_status": lvs.ShowRSStatus(cfg),
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
		if store.LVSRole(&st.LVS) == store.LVSRoleRS {
			writeBadRequest(w, "virtual servers require director role")
			return
		}
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

func (srv *Server) handleLVSRealServers(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	if store.LVSRole(&st.LVS) == store.LVSRoleRS {
		writeBadRequest(w, "real server management requires director role")
		return
	}
	switch r.Method {
	case http.MethodPost:
		var body struct {
			VSID        string                `json:"vs_id"`
			IP          string                `json:"ip,omitempty"`
			Port        int                   `json:"port,omitempty"`
			Weight      int                   `json:"weight,omitempty"`
			RealServers []store.LVSRealServer `json:"real_servers,omitempty"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.VSID = strings.TrimSpace(body.VSID)
		toAdd := body.RealServers
		if len(toAdd) == 0 {
			if body.VSID == "" || strings.TrimSpace(body.IP) == "" {
				writeBadRequest(w, "vs_id and ip (or real_servers) required")
				return
			}
			toAdd = []store.LVSRealServer{{IP: body.IP, Port: body.Port, Weight: body.Weight}}
		}
		if body.VSID == "" {
			writeBadRequest(w, "vs_id required")
			return
		}
		var updated store.LVSVirtualServer
		var opErr error
		_ = srv.store.Update(func(st *store.State) {
			for _, rs := range toAdd {
				updated, opErr = store.AddLVSRealServer(&st.LVS, body.VSID, rs, srv.env.DevWAN)
				if opErr != nil {
					return
				}
			}
		})
		if opErr != nil {
			if strings.Contains(opErr.Error(), "not found") {
				writeNotFound(w, opErr.Error())
				return
			}
			writeBadRequest(w, opErr.Error())
			return
		}
		if !srv.persistState(w) {
			return
		}
		if err := srv.applyLVSFromStore(); err != nil {
			writeApplyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	case http.MethodDelete:
		vsID := strings.TrimSpace(r.URL.Query().Get("vs_id"))
		ip := strings.TrimSpace(r.URL.Query().Get("ip"))
		if vsID == "" || ip == "" {
			writeBadRequest(w, "vs_id and ip query required")
			return
		}
		port := 0
		if ps := strings.TrimSpace(r.URL.Query().Get("port")); ps != "" {
			if _, err := fmt.Sscanf(ps, "%d", &port); err != nil {
				writeBadRequest(w, "invalid port")
				return
			}
		}
		var updated store.LVSVirtualServer
		var addErr error
		_ = srv.store.Update(func(st *store.State) {
			updated, addErr = store.RemoveLVSRealServer(&st.LVS, vsID, ip, port, srv.env.DevWAN)
		})
		if addErr != nil {
			if strings.Contains(addErr.Error(), "not found") {
				writeNotFound(w, addErr.Error())
				return
			}
			writeBadRequest(w, addErr.Error())
			return
		}
		if !srv.persistState(w) {
			return
		}
		if err := srv.applyLVSFromStore(); err != nil {
			writeApplyError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, updated)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) applyLVSFromStore() error {
	if os.Getuid() != 0 {
		return nil
	}
	st := srv.store.Get()
	if err := srv.applyLVS(st.LVS); err != nil {
		return err
	}
	srv.persistAutoFirewallRules()
	srv.tryReloadNft()
	return nil
}

func (srv *Server) applyLVS(cfg store.LVSState) error {
	return lvs.Apply(lvs.Config{DevWAN: srv.env.DevWAN, State: cfg})
}

type lvsOCServClusterRequest struct {
	VIP            string   `json:"vip"`
	Port           int      `json:"port,omitempty"`
	Nodes          []string `json:"nodes"`
	PersistenceSec int      `json:"persistence_sec,omitempty"`
	Scheduler      string   `json:"scheduler,omitempty"`
	AutoVIP        bool     `json:"auto_vip"`
	Comment        string   `json:"comment,omitempty"`
}

func (srv *Server) handleLVSOCServCluster(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	var body lvsOCServClusterRequest
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	if len(body.Nodes) == 0 {
		writeBadRequest(w, "nodes required")
		return
	}
	vs, err := store.BuildLVSOCServCluster(body.VIP, body.Port, body.Nodes, srv.env.DevWAN, body.AutoVIP, body.PersistenceSec, body.Scheduler)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if c := strings.TrimSpace(body.Comment); c != "" {
		vs.Comment = c
	}
	st := srv.store.Get()
	if store.LVSRole(&st.LVS) == store.LVSRoleRS {
		writeBadRequest(w, "ocserv cluster requires director role")
		return
	}
	if store.LVSVSConflictsForward(vs, st.Firewall.WanPortForwards) {
		writeBadRequest(w, "conflicts with WAN port forward on same vip:port")
		return
	}
	if store.LVSOCServConflictsLocal(vs, st.VPN.OCServ) {
		writeBadRequest(w, "local ocserv is enabled on the same port; disable local ocserv first")
		return
	}
	for _, existing := range st.LVS.VirtualServers {
		if existing.VIP == vs.VIP && existing.Port == vs.Port && existing.Protocol == "tcp_udp" {
			writeBadRequest(w, "ocserv cluster already exists for this vip:port")
			return
		}
	}
	_ = srv.store.Update(func(st *store.State) {
		st.LVS.VirtualServers = append(st.LVS.VirtualServers, vs)
		st.LVS.Enabled = true
		if st.LVS.Mode == "" {
			st.LVS.Mode = "nat"
		}
	})
	if !srv.persistState(w) {
		return
	}
	if err := srv.applyLVSFromStore(); err != nil {
		writeApplyError(w, err)
		return
	}
	srv.auditLog(r, "lvs.ocserv_cluster", vs.VIP)
	writeJSON(w, http.StatusOK, vs)
}
