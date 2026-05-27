package api

import (
	"net/http"
	"strings"

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
		configured := st.Nat.IPv4.SharedIPs
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
		m := srv.store.Get().Nat.IPv4.StaticMappings
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
		body.Inner = strings.TrimSpace(body.Inner)
		body.Outer = strings.TrimSpace(body.Outer)
		if err := store.ValidateIPv4OrCIDR(body.Inner); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner: " + err.Error()})
			return
		}
		if err := store.ValidateIPv4OrCIDR(body.Outer); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "outer: " + err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Nat.IPv4.StaticMappings[body.Inner] = body.Outer
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
			if _, ok := st.Nat.IPv4.StaticMappings[inner]; ok {
				delete(st.Nat.IPv4.StaticMappings, inner)
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
		m := srv.store.Get().Nat.IPv4.PrefixMappings
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
		body.Inner = strings.TrimSpace(body.Inner)
		body.Outer = strings.TrimSpace(body.Outer)
		if err := store.ValidateCIDR(body.Inner); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "inner: " + err.Error()})
			return
		}
		if err := store.ValidateCIDR(body.Outer); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "outer: " + err.Error()})
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Nat.IPv4.PrefixMappings[body.Inner] = body.Outer
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
			if _, ok := st.Nat.IPv4.PrefixMappings[inner]; ok {
				delete(st.Nat.IPv4.PrefixMappings, inner)
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
		writeJSON(w, http.StatusOK, srv.store.Get().Nat.IPv4.PolicyRoutes)
	case http.MethodPost:
		var body struct {
			CIDR string `json:"cidr"`
		}
		if err := readJSON(r, &body); err != nil || body.CIDR == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr required"})
			return
		}
		body.CIDR = strings.TrimSpace(body.CIDR)
		if err := store.ValidateCIDR(body.CIDR); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := srv.store.Update(func(st *store.State) {
			for _, c := range st.Nat.IPv4.PolicyRoutes {
				if c == body.CIDR {
					return
				}
			}
			st.Nat.IPv4.PolicyRoutes = append(st.Nat.IPv4.PolicyRoutes, body.CIDR)
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
			for _, c := range st.Nat.IPv4.PolicyRoutes {
				if c != cidr {
					out = append(out, c)
				}
			}
			st.Nat.IPv4.PolicyRoutes = out
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
