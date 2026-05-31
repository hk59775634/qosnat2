package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleWanForwards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleWanForwardsGet(w, r)
	case http.MethodPost:
		srv.handleWanForwardsPost(w, r)
	case http.MethodDelete:
		srv.handleWanForwardsDelete(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWanForwardsGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	ifaces, _ := dnsmasq.ListInterfaces()
	if ifaces == nil {
		ifaces = []dnsmasq.Iface{}
	}
	defIface := srv.env.DevWAN
	defDst := ""
	for _, iface := range ifaces {
		if iface.Name == defIface && len(iface.Addrs) > 0 {
			defDst = strings.Split(iface.Addrs[0], "/")[0]
			break
		}
	}
	forwards := st.Firewall.WanPortForwards
	if forwards == nil {
		forwards = []store.WanPortForward{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"forwards":   forwards,
		"dev_lan":    srv.env.DevLAN,
		"dev_wan":    srv.env.DevWAN,
		"interfaces": ifaces,
		"defaults": map[string]any{
			"interface":     defIface,
			"ip_version":    "ipv4",
			"src_addr":      "0.0.0.0/0",
			"src_addr_v6":   "::/0",
			"proto":         "tcp",
			"dst_addr":      defDst,
			"redirect_port": 0,
		},
	})
}

func (srv *Server) handleWanForwardsPost(w http.ResponseWriter, r *http.Request) {
	var f store.WanPortForward
	if err := readJSON(r, &f); err != nil {
		writeBadJSON(w)
		return
	}
	if err := store.NormalizeWanForward(&f, srv.env.DevWAN); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if !route.LinkExists(f.Interface) {
		writeBadRequest(w, "interface not found: "+f.Interface)
		return
	}
	st := srv.store.Get()
	proposed := st
	proposed.Firewall.WanPortForwards = append(cloneWanForwards(st.Firewall.WanPortForwards), f)
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}
	backupFwd := cloneWanForwards(st.Firewall.WanPortForwards)
	backupRules := cloneFilterRules(st.Firewall.FilterRules)
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.WanPortForwards = append(st.Firewall.WanPortForwards, f)
	})
	srv.syncAutoFirewallRules()
	if !srv.saveState(w) {
		srv.setWanForwards(backupFwd)
		srv.setFilterRules(backupRules)
		return
	}
	if err := srv.reloadNftWithForwardRevert(backupFwd, backupRules); err != nil {
		writeApplyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (srv *Server) handleWanForwardsDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeBadRequest(w, "id query required")
		return
	}
	found := false
	st := srv.store.Get()
	var newFwd []store.WanPortForward
	for _, fwd := range st.Firewall.WanPortForwards {
		if fwd.ID == id {
			found = true
			continue
		}
		newFwd = append(newFwd, fwd)
	}
	if !found {
		writeNotFound(w, "not found")
		return
	}
	proposed := st
	proposed.Firewall.WanPortForwards = newFwd
	if err := srv.checkNftForState(proposed); err != nil {
		writeNftApplyError(w, err)
		return
	}
	backupFwd := cloneWanForwards(st.Firewall.WanPortForwards)
	backupRules := cloneFilterRules(st.Firewall.FilterRules)
	srv.setWanForwards(newFwd)
	srv.syncAutoFirewallRules()
	if !srv.saveState(w) {
		srv.setWanForwards(backupFwd)
		srv.setFilterRules(backupRules)
		return
	}
	if err := srv.reloadNftWithForwardRevert(backupFwd, backupRules); err != nil {
		writeApplyError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
