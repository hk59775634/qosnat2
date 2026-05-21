package api

import (
	"log"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleNetworkVLANs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		vlans := st.Network.VLANs
		if vlans == nil {
			vlans = []store.VLANIface{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"vlans":        vlans,
			"netplan_path": netif.NetplanConfigPathForAPI(),
		})
	case http.MethodPost:
		var body store.VLANIface
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if body.ID == "" {
			body.ID = store.NewVLANID()
		}
		if body.Parent == "" || body.VID < 1 {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parent and vid required"})
			return
		}
		if !route.LinkExists(body.Parent) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parent interface not found"})
			return
		}
		body.Name = netif.VLANName(body.Parent, body.VID)
		_ = srv.store.Update(func(st *store.State) {
			st.Network.VLANs = append(st.Network.VLANs, body)
		})
		_ = srv.store.Save()
		if err := srv.applyNetplan(); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.vlan.add", body.Name)
		writeJSON(w, http.StatusOK, body)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		st := srv.store.Get()
		found := false
		for _, v := range st.Network.VLANs {
			if v.ID == id {
				found = true
				break
			}
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "vlan not found"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.VLANIface
			for _, v := range st.Network.VLANs {
				if v.ID != id {
					out = append(out, v)
				}
			}
			st.Network.VLANs = out
		})
		_ = srv.store.Save()
		if err := srv.applyNetplan(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.vlan.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleNetworkWanLinks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		links := st.Network.WanLinks
		if links == nil {
			links = []store.WanLink{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"wan_links": links,
			"dev_wan":   srv.env.DevWAN,
		})
	case http.MethodPost:
		var body store.WanLink
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if err := store.NormalizeWanLink(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !route.LinkExists(body.Device) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "interface not found"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Network.WanLinks = append(st.Network.WanLinks, body)
			store.SyncWanRoutes(st)
		})
		_ = srv.store.Save()
		srv.applyManagedRoutes()
		srv.auditLog(r, "network.wan.add", body.ID)
		writeJSON(w, http.StatusOK, body)
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		var body store.WanLink
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.ID = id
		if err := store.NormalizeWanLink(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			for i, w := range st.Network.WanLinks {
				if w.ID == id {
					st.Network.WanLinks[i] = body
					found = true
					break
				}
			}
			if found {
				store.SyncWanRoutes(st)
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "wan link not found"})
			return
		}
		_ = srv.store.Save()
		srv.applyManagedRoutes()
		srv.auditLog(r, "network.wan.put", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.WanLink
			for _, w := range st.Network.WanLinks {
				if w.ID != id {
					out = append(out, w)
				}
			}
			st.Network.WanLinks = out
			store.SyncWanRoutes(st)
		})
		_ = srv.store.Save()
		srv.applyManagedRoutes()
		srv.auditLog(r, "network.wan.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) applyNetworkVLANs() {
	if err := srv.applyNetplan(); err != nil {
		log.Printf("netplan apply: %v", err)
	}
}

func (srv *Server) replayWanLinksOnBoot() {
	st := srv.store.Get()
	if len(st.Network.WanLinks) == 0 {
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		store.SyncWanRoutes(st)
	})
	_ = srv.store.Save()
}
