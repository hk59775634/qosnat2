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
		"forwards":  forwards,
		"dev_lan":   srv.env.DevLAN,
		"dev_wan":   srv.env.DevWAN,
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
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if err := store.NormalizeWanForward(&f, srv.env.DevWAN); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if !route.LinkExists(f.Interface) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "interface not found: " + f.Interface})
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.WanPortForwards = append(st.Firewall.WanPortForwards, f)
	})
	_ = srv.store.Save()
	if err := srv.reloadNft(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, f)
}

func (srv *Server) handleWanForwardsDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
		return
	}
	found := false
	_ = srv.store.Update(func(st *store.State) {
		var out []store.WanPortForward
		for _, f := range st.Firewall.WanPortForwards {
			if f.ID == id {
				found = true
				continue
			}
			out = append(out, f)
		}
		st.Firewall.WanPortForwards = out
	})
	if !found {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	_ = srv.store.Save()
	if err := srv.reloadNft(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
