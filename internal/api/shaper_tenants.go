package api

import (
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleShaperTenants(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		list := st.Shaper.Tenants
		if list == nil {
			list = []store.TenantEntry{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"tenants": list})
	case http.MethodPost:
		var body store.TenantEntry
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeTenant(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if !srv.bpfReady() {
			writeUnavailable(w, "", errEbpfNotLoaded.Error())
			return
		}
		if err := srv.applyTenantProfiles(body, false); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Shaper.Tenants = append(st.Shaper.Tenants, body)
		})
		if !srv.persistState(w) {
			return
		}
		srv.refreshShaperAfterChange()
		srv.auditLog(r, "shaper.tenant.add", body.Name)
		writeJSON(w, http.StatusOK, body)
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		var body store.TenantEntry
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.ID = id
		if err := store.NormalizeTenant(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		var prev store.TenantEntry
		found := false
		st := srv.store.Get()
		for _, t := range st.Shaper.Tenants {
			if t.ID == id {
				prev = t
				found = true
				break
			}
		}
		if !found {
			writeNotFound(w, "tenant not found")
			return
		}
		if err := srv.applyTenantProfiles(body, false); err != nil {
			_ = srv.applyTenantProfiles(prev, false)
			srv.refreshShaperAfterChange()
			writeBadRequest(w, err.Error())
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			for i, t := range st.Shaper.Tenants {
				if t.ID == id {
					st.Shaper.Tenants[i] = body
					break
				}
			}
		})
		if !srv.persistState(w) {
			return
		}
		srv.refreshShaperAfterChange()
		srv.auditLog(r, "shaper.tenant.put", body.Name)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "tenant": body})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			var out []store.TenantEntry
			for _, t := range st.Shaper.Tenants {
				if t.ID == id {
					found = true
					continue
				}
				out = append(out, t)
			}
			st.Shaper.Tenants = out
		})
		if !found {
			writeNotFound(w, "tenant not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		srv.removeTenantProfiles(id)
		srv.refreshShaperAfterChange()
		srv.auditLog(r, "shaper.tenant.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) applyTenantProfiles(t store.TenantEntry, refreshEach bool) error {
	srv.removeTenantProfiles(t.ID)
	dev := strings.TrimSpace(t.Device)
	for _, cidr := range t.CIDRs {
		mask := 32
		if _, ipNet, err := net.ParseCIDR(cidr); err == nil && ipNet != nil {
			ones, _ := ipNet.Mask.Size()
			if ones < 32 {
				mask = ones
			}
		}
		if _, err := srv.upsertShaperProfile(cidr, t.Down, t.Up, mask, dev, false, refreshEach); err != nil {
			return err
		}
		_ = srv.store.Update(func(st *store.State) {
			for i := range st.Shaper.Profiles {
				if st.Shaper.Profiles[i].CIDR == cidr {
					st.Shaper.Profiles[i].TenantID = t.ID
					break
				}
			}
		})
		if err := srv.store.Save(); err != nil {
			return fmt.Errorf("save tenant profile: %w", err)
		}
	}
	return nil
}

func (srv *Server) removeTenantProfiles(tenantID string) {
	if tenantID == "" {
		return
	}
	st := srv.store.Get()
	var toDel []string
	for _, p := range st.Shaper.Profiles {
		if p.TenantID == tenantID {
			toDel = append(toDel, p.CIDR)
		}
	}
	for _, cidr := range toDel {
		srv.teardownProfileShaper(cidr)
		_ = srv.store.Update(func(st *store.State) {
			var out []store.ProfileEntry
			for _, p := range st.Shaper.Profiles {
				if p.CIDR != cidr {
					out = append(out, p)
				}
			}
			st.Shaper.Profiles = out
		})
	}
	_ = srv.persistStateOrLog("remove tenant profiles")
}
