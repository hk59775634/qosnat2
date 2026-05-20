package api

import (
	"log"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleRoutesGet(w, r)
	case http.MethodPost:
		srv.handleRoutesPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleRoutesGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	live, err := route.ListLive(254)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	live = route.MarkManaged(live, st.Routes)
	managed := st.Routes
	if managed == nil {
		managed = []store.RouteEntry{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"managed":  managed,
		"live":     live,
		"dev_lan":  srv.env.DevLAN,
		"dev_wan":  srv.env.DevWAN,
		"table":    254,
	})
}

func (srv *Server) handleRoutesPost(w http.ResponseWriter, r *http.Request) {
	var body store.RouteEntry
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	entry, err := srv.normalizeRouteInput(body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := route.Apply(entry); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		st.Routes = append(st.Routes, entry)
	})
	_ = srv.store.Save()
	writeJSON(w, http.StatusOK, entry)
}

func (srv *Server) handleRoutesItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/routes/")
	if id == "" || strings.Contains(id, "/") {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body store.RouteEntry
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		entry, err := srv.normalizeRouteInput(body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		var old store.RouteEntry
		found := false
		_ = srv.store.Update(func(st *store.State) {
			for i, e := range st.Routes {
				if e.ID == id {
					old = e
					entry.ID = id
					st.Routes[i] = entry
					found = true
					break
				}
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		_ = route.Delete(old)
		if entry.Enabled {
			if err := route.Apply(entry); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, entry)
	case http.MethodDelete:
		var old store.RouteEntry
		found := false
		_ = srv.store.Update(func(st *store.State) {
			var out []store.RouteEntry
			for _, e := range st.Routes {
				if e.ID == id {
					old = e
					found = true
					continue
				}
				out = append(out, e)
			}
			st.Routes = out
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		_ = route.Delete(old)
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleRoutesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	if err := route.ApplyAll(st.Routes); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) normalizeRouteInput(body store.RouteEntry) (store.RouteEntry, error) {
	dest, err := store.NormalizeRouteDest(body.Dest)
	if err != nil {
		return store.RouteEntry{}, err
	}
	dev := strings.TrimSpace(body.Device)
	if dev == "" && dest == "default" {
		dev = srv.env.DevWAN
	}
	if dev != "" && !route.LinkExists(dev) {
		return store.RouteEntry{}, errDeviceNotFound(dev)
	}
	gw := strings.TrimSpace(body.Gateway)
	if dest != "default" && gw == "" && dev == "" {
		return store.RouteEntry{}, errNeedGwOrDev()
	}
	entry := store.RouteEntry{
		ID:      body.ID,
		Dest:    dest,
		Gateway: gw,
		Device:  dev,
		Table:   body.Table,
		Metric:  body.Metric,
		Scope:   strings.TrimSpace(body.Scope),
		Comment: strings.TrimSpace(body.Comment),
		Enabled: body.Enabled,
	}
	if entry.ID == "" {
		entry.Enabled = true
		entry.ID = store.NewRouteID()
	}
	return entry, nil
}

type errDevice string

func (e errDevice) Error() string { return "interface not found: " + string(e) }
func errDeviceNotFound(name string) error { return errDevice(name) }

type errGwDev struct{}

func (errGwDev) Error() string { return "non-default route requires gateway or device" }
func errNeedGwOrDev() error { return errGwDev{} }

func (srv *Server) applyManagedRoutes() {
	st := srv.store.Get()
	if err := route.ApplyAll(st.Routes); err != nil {
		log.Printf("routes apply: %v", err)
	}
}
