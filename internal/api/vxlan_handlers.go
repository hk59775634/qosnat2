package api

import (
	"fmt"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleNetworkVXLAN(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		list := st.Network.VXLANTunnels
		if list == nil {
			list = []store.VXLANTunnel{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"vxlan_tunnels": list,
			"netplan_path":  netif.NetplanConfigPathForAPI(),
		})
	case http.MethodPost:
		var body store.VXLANTunnel
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeVXLANTunnel(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if body.Underlay != "" && !route.LinkExists(body.Underlay) {
			writeBadRequest(w, "underlay interface not found")
			return
		}
		if err := srv.applyNetplanWithRollback(func(st *store.State) error {
			st.Network.VXLANTunnels = append(st.Network.VXLANTunnels, body)
			return nil
		}); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		srv.auditLog(r, "network.vxlan.add", body.Name)
		writeJSON(w, http.StatusOK, body)
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		var body store.VXLANTunnel
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.ID = id
		if err := store.NormalizeVXLANTunnel(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		found := false
		for _, v := range srv.store.Get().Network.VXLANTunnels {
			if v.ID == id {
				found = true
				break
			}
		}
		if !found {
			writeNotFound(w, "vxlan not found")
			return
		}
		if err := srv.applyNetplanWithRollback(func(st *store.State) error {
			for i, v := range st.Network.VXLANTunnels {
				if v.ID == id {
					st.Network.VXLANTunnels[i] = body
					return nil
				}
			}
			return fmt.Errorf("vxlan not found")
		}); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		srv.auditLog(r, "network.vxlan.put", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tunnel": body})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		st := srv.store.Get()
		found := false
		for _, v := range st.Network.VXLANTunnels {
			if v.ID == id {
				found = true
				break
			}
		}
		if !found {
			writeNotFound(w, "vxlan not found")
			return
		}
		if err := srv.applyNetplanWithRollback(func(st *store.State) error {
			var out []store.VXLANTunnel
			for _, v := range st.Network.VXLANTunnels {
				if v.ID != id {
					out = append(out, v)
				}
			}
			st.Network.VXLANTunnels = out
			return nil
		}); err != nil {
			writeInternalError(w, err.Error())
			return
		}
		srv.auditLog(r, "network.vxlan.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
