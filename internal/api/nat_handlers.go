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
			writeBadRequest(w, "ip required")
			return
		}
		trial := srv.store.Get()
		if err := nft.AddSharedIP(&trial, body.IP); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			_ = nft.AddSharedIP(st, body.IP)
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		ip := r.URL.Query().Get("ip")
		if ip == "" {
			writeBadRequest(w, "ip query required")
			return
		}
		found := false
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			found = nft.RemoveSharedIP(st, ip)
		}) {
			return
		}
		if !found {
			writeNotFound(w, "not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
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
			writeBadRequest(w, "inner/outer required")
			return
		}
		body.Inner = strings.TrimSpace(body.Inner)
		body.Outer = strings.TrimSpace(body.Outer)
		if err := store.ValidateIPv4OrCIDR(body.Inner); err != nil {
			writeBadRequest(w, "inner: "+err.Error())
			return
		}
		if err := store.ValidateIPv4OrCIDR(body.Outer); err != nil {
			writeBadRequest(w, "outer: "+err.Error())
			return
		}
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			st.Nat.IPv4.StaticMappings[body.Inner] = body.Outer
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		inner := r.URL.Query().Get("inner")
		if inner == "" {
			writeBadRequest(w, "inner query required")
			return
		}
		found := false
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			if _, ok := st.Nat.IPv4.StaticMappings[inner]; ok {
				delete(st.Nat.IPv4.StaticMappings, inner)
				found = true
			}
		}) {
			return
		}
		if !found {
			writeNotFound(w, "not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
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
			writeBadRequest(w, "inner/outer required")
			return
		}
		body.Inner = strings.TrimSpace(body.Inner)
		body.Outer = strings.TrimSpace(body.Outer)
		if err := store.ValidateCIDR(body.Inner); err != nil {
			writeBadRequest(w, "inner: "+err.Error())
			return
		}
		if err := store.ValidateCIDR(body.Outer); err != nil {
			writeBadRequest(w, "outer: "+err.Error())
			return
		}
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			st.Nat.IPv4.PrefixMappings[body.Inner] = body.Outer
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		inner := r.URL.Query().Get("inner")
		if inner == "" {
			writeBadRequest(w, "inner query required")
			return
		}
		found := false
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			if _, ok := st.Nat.IPv4.PrefixMappings[inner]; ok {
				delete(st.Nat.IPv4.PrefixMappings, inner)
				found = true
			}
		}) {
			return
		}
		if !found {
			writeNotFound(w, "not found")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleNatIPv4(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"ipv4":    st.Nat.IPv4,
			"enabled": store.NatIPv4Enabled(st.Nat.IPv4),
		})
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		enabled := body.Enabled
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			st.Nat.IPv4.Enabled = &enabled
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
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
			writeBadRequest(w, "cidr required")
			return
		}
		body.CIDR = strings.TrimSpace(body.CIDR)
		if err := store.ValidateCIDR(body.CIDR); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		dup := false
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			for _, c := range st.Nat.IPv4.PolicyRoutes {
				if c == body.CIDR {
					dup = true
					return
				}
			}
			st.Nat.IPv4.PolicyRoutes = append(st.Nat.IPv4.PolicyRoutes, body.CIDR)
		}) {
			return
		}
		if dup {
			writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		cidr := r.URL.Query().Get("cidr")
		if cidr == "" {
			writeBadRequest(w, "cidr query required")
			return
		}
		if !srv.commitNatIPv4Change(w, func(st *store.State) {
			var out []string
			for _, c := range st.Nat.IPv4.PolicyRoutes {
				if c != cidr {
					out = append(out, c)
				}
			}
			st.Nat.IPv4.PolicyRoutes = out
		}) {
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}
