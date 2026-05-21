package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/geoip"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFirewallGeoIP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		rules := st.Firewall.GeoIP
		if rules == nil {
			rules = []store.GeoIPRule{}
		}
		_ = geoip.EnsureDataDir()
		writeJSON(w, http.StatusOK, map[string]any{
			"rules":    rules,
			"data_dir": geoip.DataDir,
			"dev_wan":  srv.env.DevWAN,
		})
	case http.MethodPost:
		var body store.GeoIPRule
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if err := store.NormalizeGeoIP(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			st.Firewall.GeoIP = append(st.Firewall.GeoIP, body)
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.geoip.add", body.Country)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": body.ID})
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
			return
		}
		var body store.GeoIPRule
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.ID = id
		if err := store.NormalizeGeoIP(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			for i, rule := range st.Firewall.GeoIP {
				if rule.ID == id {
					st.Firewall.GeoIP[i] = body
					found = true
					break
				}
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "rule not found"})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.geoip.put", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id query required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			var out []store.GeoIPRule
			for _, rule := range st.Firewall.GeoIP {
				if rule.ID != id {
					out = append(out, rule)
				}
			}
			st.Firewall.GeoIP = out
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "firewall.geoip.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
