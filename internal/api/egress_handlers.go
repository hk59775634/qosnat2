package api

import (
	"log"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/policyroute"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleNetworkEgressPolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		policies := st.Network.EgressPolicies
		if policies == nil {
			policies = []store.EgressPolicy{}
		}
		links := st.Network.WanLinks
		if links == nil {
			links = []store.WanLink{}
		}
		resolved := store.ResolveEgressPolicies(st, netif.PrimaryIPv4)
		writeJSON(w, http.StatusOK, map[string]any{
			"egress_policies": policies,
			"wan_links":       links,
			"resolved":        resolved,
			"dev_wan":         srv.env.DevWAN,
		})
	case http.MethodPost:
		var body store.EgressPolicy
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		if err := store.NormalizeEgressPolicy(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		st := srv.store.Get()
		if _, ok := store.FindWanLink(st.Network.WanLinks, body.WanLinkID); !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "wan_link_id not found"})
			return
		}
		for _, p := range st.Network.EgressPolicies {
			if p.CIDR == body.CIDR && p.ID != body.ID {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr already used by another egress policy"})
				return
			}
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Network.EgressPolicies = append(st.Network.EgressPolicies, body)
			store.SyncEgressRoutes(st)
		})
		_ = srv.store.Save()
		if err := srv.applyEgressDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.egress.add", body.ID)
		writeJSON(w, http.StatusOK, body)
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		var body store.EgressPolicy
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.ID = id
		if err := store.NormalizeEgressPolicy(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		stBefore := srv.store.Get()
		if _, ok := store.FindWanLink(stBefore.Network.WanLinks, body.WanLinkID); !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "wan_link_id not found"})
			return
		}
		for _, p := range stBefore.Network.EgressPolicies {
			if p.ID != id && p.CIDR == body.CIDR {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cidr already used"})
				return
			}
		}
		var old store.EgressPolicy
		found := false
		_ = srv.store.Update(func(st *store.State) {
			for i, p := range st.Network.EgressPolicies {
				if p.ID == id {
					old = p
					st.Network.EgressPolicies[i] = body
					found = true
					break
				}
			}
			if found {
				store.SyncEgressRoutes(st)
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "egress policy not found"})
			return
		}
		if old.ID != "" {
			policyroute.DeletePolicy(old, srv.store.Get().Network.WanLinks)
		}
		_ = srv.store.Save()
		if err := srv.applyEgressDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.egress.put", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		var removed store.EgressPolicy
		var links []store.WanLink
		found := false
		_ = srv.store.Update(func(st *store.State) {
			links = append([]store.WanLink(nil), st.Network.WanLinks...)
			var out []store.EgressPolicy
			for _, p := range st.Network.EgressPolicies {
				if p.ID == id {
					removed = p
					found = true
					continue
				}
				out = append(out, p)
			}
			if found {
				st.Network.EgressPolicies = out
				store.SyncEgressRoutes(st)
			}
		})
		if !found {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "egress policy not found"})
			return
		}
		policyroute.DeletePolicy(removed, links)
		_ = srv.store.Save()
		if err := srv.applyEgressDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		srv.auditLog(r, "network.egress.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) applyEgressDataPlane() error {
	if !srv.store.Get().SetupComplete {
		return nil
	}
	_ = srv.store.Update(func(st *store.State) {
		store.SyncEgressRoutes(st)
	})
	_ = srv.store.Save()
	srv.applyManagedRoutes()
	if err := policyroute.Apply(srv.store.Get()); err != nil {
		return err
	}
	return srv.reloadNft()
}

func (srv *Server) replayEgressOnBoot() {
	st := srv.store.Get()
	if len(st.Network.EgressPolicies) == 0 {
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		store.SyncEgressRoutes(st)
	})
	_ = srv.store.Save()
}

// applyEgressPolicyRoutes 回放 ip rule（applyState / 手动 apply 时调用）
func (srv *Server) applyEgressPolicyRoutes() {
	if !srv.store.Get().SetupComplete {
		return
	}
	if err := policyroute.Apply(srv.store.Get()); err != nil {
		log.Printf("egress policy routes: %v", err)
	}
}

func egressPoliciesUsingWanLink(st store.State, wanID string) []store.EgressPolicy {
	var out []store.EgressPolicy
	for _, p := range st.Network.EgressPolicies {
		if p.WanLinkID == wanID {
			out = append(out, p)
		}
	}
	return out
}

func validateWanLinkDeletable(st store.State, wanID string) error {
	if len(egressPoliciesUsingWanLink(st, wanID)) > 0 {
		return errWanLinkInUse{wanID: wanID}
	}
	return nil
}

type errWanLinkInUse struct{ wanID string }

func (e errWanLinkInUse) Error() string {
	return "wan link in use by egress policy: " + e.wanID
}
