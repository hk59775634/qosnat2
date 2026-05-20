package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleSharedIPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		cfg := srv.nftCfg()
		ips, autoWAN := nft.ResolveSharedIPs(cfg, st)
		if ips == nil {
			ips = []string{}
		}
		configured := st.SharedIPs
		if configured == nil {
			configured = []string{}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"ips":           ips,
			"configured":    configured,
			"auto_from_wan": autoWAN,
			"wan_device":    srv.env.DevWAN,
		})
	case http.MethodPost:
		var body struct {
			IP string `json:"ip"`
		}
		if err := readJSON(r, &body); err != nil || body.IP == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip required"})
			return
		}
		var addErr error
		_ = srv.store.Update(func(st *store.State) {
			addErr = nft.AddSharedIP(st, body.IP)
		})
		if addErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": addErr.Error()})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ip query required"})
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			found = nft.RemoveSharedIP(st, ip)
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleStaticMappings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m := srv.store.Get().StaticMappings
		if m == nil {
			m = map[string]string{}
		}
		writeJSON(w, http.StatusOK, m)
	case http.MethodPost:
		var body struct {
			Inner string `json:"inner"`
			Outer string `json:"outer"`
		}
		if err := readJSON(r, &body); err != nil || body.Inner == "" || body.Outer == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner/outer required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.StaticMappings[body.Inner] = body.Outer
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		inner := r.URL.Query().Get("inner")
		if inner == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner query required"})
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			if _, ok := st.StaticMappings[inner]; ok {
				delete(st.StaticMappings, inner)
				found = true
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handlePrefixMappings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		m := srv.store.Get().PrefixMappings
		if m == nil {
			m = map[string]string{}
		}
		writeJSON(w, http.StatusOK, m)
	case http.MethodPost:
		var body struct {
			Inner string `json:"inner"`
			Outer string `json:"outer"`
		}
		if err := readJSON(r, &body); err != nil || body.Inner == "" || body.Outer == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner/outer required"})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.PrefixMappings[body.Inner] = body.Outer
		})
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		inner := r.URL.Query().Get("inner")
		if inner == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner query required"})
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			if _, ok := st.PrefixMappings[inner]; ok {
				delete(st.PrefixMappings, inner)
				found = true
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handlePolicyRoutes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().PolicyRoutes)
	case http.MethodPost:
		var body struct {
			CIDR string `json:"cidr"`
		}
		if err := readJSON(r, &body); err != nil || body.CIDR == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr required"})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			for _, c := range st.PolicyRoutes {
				if c == body.CIDR {
					return
				}
			}
			st.PolicyRoutes = append(st.PolicyRoutes, body.CIDR)
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		if err := srv.reloadNft(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr query required"})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			var out []string
			for _, c := range st.PolicyRoutes {
				if c != cidr {
					out = append(out, c)
				}
			}
			st.PolicyRoutes = out
		}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		_ = srv.store.Save()
		_ = srv.reloadNft()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
