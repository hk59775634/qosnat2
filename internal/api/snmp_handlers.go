package api

import (
	"net/http"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/snmpd"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleSNMP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		cfg := st.SNMP
		_ = store.NormalizeSNMP(&cfg)
		pub := cfg
		pub.ROCommunity = ""
		rendered := ""
		if snmpd.ShowStatus().Installed {
			rendered = snmpd.RenderConf(cfg)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"config":            pub,
			"ro_community_set":  strings.TrimSpace(cfg.ROCommunity) != "",
			"status":            snmpd.ShowStatus(),
			"rendered":          rendered,
			"root":              os.Getuid() == 0,
			"monitoring":        snmpd.MonitoringHintsFor(srv.env.DevLAN, srv.env.DevWAN),
		})
	case http.MethodPut:
		var body store.SNMPState
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeSNMP(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		prev := srv.store.Get().SNMP
		_ = srv.store.Update(func(st *store.State) {
			st.SNMP = body
		})
		if !srv.persistState(w) {
			return
		}
		if store.SNMPFirewallChanged(prev, body) {
			if warn := srv.tryReloadNft(); warn != "" {
				writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nft_warning": warn})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleSNMPApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "应用 SNMP 需要 root 运行 qosnatd")
		return
	}
	st := srv.store.Get()
	cfg := st.SNMP
	if err := store.NormalizeSNMP(&cfg); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := snmpd.Apply(cfg); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	if warn := srv.tryReloadNft(); warn != "" {
		srv.auditLog(r, "snmp.apply", "")
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":          true,
			"active":      snmpd.ShowStatus().Active,
			"nft_warning": warn,
		})
		return
	}
	srv.auditLog(r, "snmp.apply", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"active": snmpd.ShowStatus().Active,
	})
}

func (srv *Server) handleSNMPService(w http.ResponseWriter, r *http.Request) {
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
	st := srv.store.Get()
	cfg := st.SNMP
	if err := store.NormalizeSNMP(&cfg); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := snmpd.ServiceAction(action, cfg); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.auditLog(r, "snmp.service."+action, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": snmpd.ShowStatus(),
	})
}

func (srv *Server) handleSNMPInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeForbidden(w, "", "安装 snmpd 需要 root 运行 qosnatd")
		return
	}
	if err := snmpd.Install(); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.auditLog(r, "snmp.install", "")
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"status": snmpd.ShowStatus(),
	})
}
