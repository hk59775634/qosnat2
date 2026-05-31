package api

import (
	"log"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleDHCP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleDHCPGet(w, r)
	case http.MethodPut:
		srv.handleDHCPPut(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleDHCPGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	cfg := st.DHCP
	if cfg.Interface == "" {
		cfg.Interface = srv.env.DevLAN
	}
	_ = store.NormalizeDHCP(&cfg, srv.env.DevLAN)
	ifaces, err := dnsmasq.ListInterfaces()
	if err != nil {
		ifaces = []dnsmasq.Iface{}
	}
	rendered := ""
	if dnsmasq.ShowStatus().Installed {
		rendered = dnsmasq.RenderConf(cfg, srv.dnsmasqOpts(st))
	}
	dnsSt := dnsmasq.ShowStatus()
	leases := dnsmasq.ParseLeases(dnsSt.LeasesRaw)
	if leases == nil {
		leases = []dnsmasq.LeaseEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"config":     cfg,
		"status":     dnsSt,
		"leases":     leases,
		"interfaces": ifaces,
		"dev_lan":    srv.env.DevLAN,
		"dev_wan":    srv.env.DevWAN,
		"rendered":   rendered,
	})
}

func (srv *Server) handleDHCPPut(w http.ResponseWriter, r *http.Request) {
	var body store.DHCPState
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	if err := srv.normalizeDHCP(&body); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		st.DHCP = body
	})
	if !srv.persistState(w) {
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) handleDHCPApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	cfg := st.DHCP
	if cfg.Interface == "" {
		cfg.Interface = srv.env.DevLAN
	}
	if err := srv.normalizeDHCP(&cfg); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := dnsmasq.Apply(cfg, srv.dnsmasqOpts(st)); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"active": dnsmasq.ShowStatus().Active,
	})
}

func (srv *Server) normalizeDHCP(d *store.DHCPState) error {
	if d.Interface == "" {
		d.Interface = srv.env.DevLAN
	}
	if (d.Enabled || d.DNSEnabled) && !route.LinkExists(d.Interface) {
		return errDeviceNotFound(d.Interface)
	}
	return store.NormalizeDHCP(d, srv.env.DevLAN)
}

func (srv *Server) applyManagedDHCP() {
	st := srv.store.Get()
	cfg := st.DHCP
	if cfg.Interface == "" {
		cfg.Interface = srv.env.DevLAN
	}
	if err := store.NormalizeDHCP(&cfg, srv.env.DevLAN); err != nil {
		log.Printf("dhcp normalize: %v", err)
		return
	}
	if err := dnsmasq.Apply(cfg, srv.dnsmasqOpts(st)); err != nil {
		log.Printf("dhcp apply: %v", err)
	}
}
