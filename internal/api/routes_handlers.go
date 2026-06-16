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
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleRoutesGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	live, err := route.ListLive(254)
	if err != nil {
		writeUnavailable(w, "", err.Error())
		return
	}
	live = route.MarkManaged(live, st.Routes)
	managed := store.EnrichManagedRoutes(st.Routes, st)
	writeJSON(w, http.StatusOK, map[string]any{
		"managed":       managed,
		"live":          live,
		"dev_lan":       srv.env.DevLAN,
		"dev_wan":       srv.env.DevWAN,
		"table":         254,
		"route_backend": route.NormalizeBackend(st.System.RouteBackend),
	})
}

func (srv *Server) routeBackend() string {
	return route.NormalizeBackend(srv.store.Get().System.RouteBackend)
}

func (srv *Server) applyManagedRoutesErr() error {
	_, err := route.ApplyFromState(srv.store.Get())
	return err
}

func (srv *Server) handleRoutesPost(w http.ResponseWriter, r *http.Request) {
	var body store.RouteEntry
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	entry, err := srv.normalizeRouteInput(body)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if srv.routeBackend() == route.BackendFRR {
		_ = srv.store.Update(func(st *store.State) {
			st.Routes = append(st.Routes, entry)
		})
		if !srv.persistState(w) {
			return
		}
		if err := srv.applyManagedRoutesErr(); err != nil {
			srv.removeRouteByID(entry.ID)
			writeInternalError(w, err.Error())
			return
		}
	} else {
		_ = srv.store.Update(func(st *store.State) {
			st.Routes = append(st.Routes, entry)
		})
		if !srv.persistState(w) {
			return
		}
		if err := route.Apply(entry); err != nil {
			srv.removeRouteByID(entry.ID)
			writeInternalError(w, err.Error())
			return
		}
	}
	st := srv.store.Get()
	writeJSON(w, http.StatusOK, store.EnrichRouteEntry(entry, st))
}

func (srv *Server) handleRoutesItem(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/routes/")
	if id == "" || strings.Contains(id, "/") {
		writeBadRequest(w, "id required")
		return
	}
	switch r.Method {
	case http.MethodPut:
		var body store.RouteEntry
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		entry, err := srv.normalizeRouteInput(body)
		if err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		entry.ID = id
		var old store.RouteEntry
		found := false
		for _, e := range srv.store.Get().Routes {
			if e.ID == id {
				old = e
				found = true
				break
			}
		}
		if !found {
			writeNotFound(w, "not found")
			return
		}
		if store.IsAutoManagedRoute(old) {
			writeForbidden(w, "ROUTE_AUTO_MANAGED", "此路由由多 WAN 或策略出站自动同步，请在「多 WAN」页面修改；本页不可编辑")
			return
		}
		if srv.routeBackend() == route.BackendFRR {
			prev, err := store.CloneState(srv.store.Get())
			if err != nil {
				writeInternalError(w, err.Error())
				return
			}
			updated := false
			_ = srv.store.Update(func(st *store.State) {
				for i, e := range st.Routes {
					if e.ID == id {
						st.Routes[i] = entry
						updated = true
						break
					}
				}
			})
			if !updated {
				writeNotFound(w, "not found")
				return
			}
			if err := srv.store.Save(); err != nil {
				writeInternalError(w, err.Error())
				return
			}
			if err := srv.applyManagedRoutesErr(); err != nil {
				srv.store.ReplaceState(prev)
				_ = srv.store.Save()
				_ = srv.applyManagedRoutesErr()
				writeInternalError(w, err.Error())
				return
			}
			st := srv.store.Get()
			writeJSON(w, http.StatusOK, store.EnrichRouteEntry(entry, st))
			return
		}
		prev, err := store.CloneState(srv.store.Get())
		if err != nil {
			writeInternalError(w, err.Error())
			return
		}
		if old.Enabled {
			if err := route.Delete(old); err != nil {
				writeInternalError(w, err.Error())
				return
			}
		}
		if entry.Enabled {
			if err := route.Apply(entry); err != nil {
				if old.Enabled {
					_ = route.Apply(old)
				}
				writeInternalError(w, err.Error())
				return
			}
		}
		updated := false
		_ = srv.store.Update(func(st *store.State) {
			for i, e := range st.Routes {
				if e.ID == id {
					st.Routes[i] = entry
					updated = true
					break
				}
			}
		})
		if !updated {
			if old.Enabled {
				_ = route.Apply(old)
			}
			if entry.Enabled {
				_ = route.Delete(entry)
			}
			writeNotFound(w, "not found")
			return
		}
		if err := srv.store.Save(); err != nil {
			srv.store.ReplaceState(prev)
			if old.Enabled {
				_ = route.Apply(old)
			}
			if entry.Enabled {
				_ = route.Delete(entry)
			}
			writeInternalError(w, err.Error())
			return
		}
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, store.EnrichRouteEntry(entry, st))
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
			writeNotFound(w, "not found")
			return
		}
		if store.IsAutoManagedRoute(old) {
			writeForbidden(w, "ROUTE_AUTO_MANAGED", "此路由由多 WAN 或策略出站自动同步，请在「多 WAN」页面删除对应链路或出站策略；本页不可删除")
			return
		}
		if srv.routeBackend() == route.BackendFRR {
			if !srv.persistState(w) {
				return
			}
			_ = route.Delete(old)
			if err := srv.applyManagedRoutesErr(); err != nil {
				writeInternalError(w, err.Error())
				return
			}
		} else {
			if !srv.persistState(w) {
				_ = srv.store.Update(func(st *store.State) {
					st.Routes = append(st.Routes, old)
				})
				return
			}
			_ = route.Delete(old)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) removeRouteByID(id string) {
	_ = srv.store.Update(func(st *store.State) {
		var out []store.RouteEntry
		for _, r := range st.Routes {
			if r.ID != id {
				out = append(out, r)
			}
		}
		st.Routes = out
	})
	_ = srv.store.Save()
}

func (srv *Server) handleRoutesApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	res, err := route.ApplyFromState(st)
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "result": res})
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
		ID:       body.ID,
		Dest:     dest,
		Gateway:  gw,
		Device:   dev,
		Nexthops: append([]store.RouteNexthop(nil), body.Nexthops...),
		Table:    body.Table,
		Metric:   body.Metric,
		Scope:    strings.TrimSpace(body.Scope),
		Comment:  strings.TrimSpace(body.Comment),
		Enabled:  body.Enabled,
	}
	if entry.ID == "" {
		entry.Enabled = true
		entry.ID = store.NewRouteID()
	}
	entry = route.InferRouteDevices(entry)
	if entry.Device != "" && !route.LinkExists(entry.Device) {
		return store.RouteEntry{}, errDeviceNotFound(entry.Device)
	}
	for _, nh := range entry.Nexthops {
		if d := strings.TrimSpace(nh.Device); d != "" && !route.LinkExists(d) {
			return store.RouteEntry{}, errDeviceNotFound(d)
		}
	}
	return entry, nil
}

type errDevice string

func (e errDevice) Error() string         { return "interface not found: " + string(e) }
func errDeviceNotFound(name string) error { return errDevice(name) }

type errGwDev struct{}

func (errGwDev) Error() string { return "non-default route requires gateway or device" }
func errNeedGwOrDev() error    { return errGwDev{} }

func (srv *Server) applyManagedRoutes() {
	if err := srv.applyManagedRoutesErr(); err != nil {
		log.Printf("routes apply: %v", err)
	}
}
