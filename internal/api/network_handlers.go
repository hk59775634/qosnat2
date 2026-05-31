package api

import (
	"fmt"
	"log"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
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
			"cloud_init_note": "基线网口可由 /etc/netplan/50-cloud-init.yaml 提供；本页变更写入 99-qosnat2.yaml",
		})
	case http.MethodPost:
		var body store.VLANIface
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if err := srv.validateVLANBody(&body, true); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.applyNetplanWithRollback(func(st *store.State) error {
			if body.ID == "" {
				body.ID = store.NewVLANID()
			}
			st.Network.VLANs = append(st.Network.VLANs, body)
			return nil
		}); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.vlan.add", body.Name)
		writeJSON(w, http.StatusOK, body)
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
			return
		}
		var body store.VLANIface
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.ID = id
		if err := srv.validateVLANBody(&body, false); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		found := false
		if err := srv.applyNetplanWithRollback(func(st *store.State) error {
			for i, v := range st.Network.VLANs {
				if v.ID == id {
					st.Network.VLANs[i] = body
					found = true
					return nil
				}
			}
			return nil
		}); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "vlan not found"})
			return
		}
		srv.auditLog(r, "network.vlan.put", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "vlan": body})
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
		if err := srv.applyNetplanWithRollback(func(st *store.State) error {
			var out []store.VLANIface
			for _, v := range st.Network.VLANs {
				if v.ID != id {
					out = append(out, v)
				}
			}
			st.Network.VLANs = out
			return nil
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.vlan.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) validateVLANBody(v *store.VLANIface, newID bool) error {
	if v.Parent == "" || v.VID < 1 || v.VID > 4094 {
		return fmt.Errorf("parent and vid 1-4094 required")
	}
	if !route.LinkExists(v.Parent) {
		return errDeviceNotFound(v.Parent + " not found")
	}
	v.Name = netif.VLANName(v.Parent, v.VID)
	if newID && v.ID == "" {
		v.ID = store.NewVLANID()
	}
	return nil
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
		if body.ID == store.WanLinkIDWarp || body.WarpManaged {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "use WARP connect to create the WARP WAN link"})
			return
		}
		if !route.LinkExists(body.Device) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "interface not found"})
			return
		}
		stCheck := srv.store.Get()
		for _, wl := range stCheck.Network.WanLinks {
			if wl.ID == body.ID {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "wan link id already exists"})
				return
			}
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Network.WanLinks = append(st.Network.WanLinks, body)
			store.SyncWanRoutes(st)
			store.SyncEgressRoutes(st)
		})
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		srv.applyManagedRoutes()
		if err := srv.applyWanLinkDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
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
		if err := validateWanLinkMutable(srv.store.Get(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
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
				store.SyncEgressRoutes(st)
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "wan link not found"})
			return
		}
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		srv.applyManagedRoutes()
		if err := srv.applyWanLinkDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.wan.put", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		if err := validateWanLinkDeletable(srv.store.Get(), id); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
			store.SyncEgressRoutes(st)
		})
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		srv.applyManagedRoutes()
		if err := srv.applyWanLinkDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
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
	warpnetns.Reconcile()
	st := srv.store.Get()
	if len(st.Network.WanLinks) == 0 && !warpnetns.IsConnected() {
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		if warpnetns.IsConnected() {
			iface := warpnetns.HostInterface()
			if iface != "" {
				store.UpsertWarpWanLink(st, iface)
			}
		} else {
			store.RemoveWarpWanLink(st)
		}
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
}
