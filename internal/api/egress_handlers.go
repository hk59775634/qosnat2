package api

import (
	"fmt"
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
			"egress_policies":            policies,
			"wan_links":                  links,
			"resolved":                   resolved,
			"dev_wan":                    srv.env.DevWAN,
			"cloudflare_cdn_cidrs_ipv4": store.CloudflareCDNCIDRsV4(),
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

// handleNetworkEgressPoliciesBulk 原子批量添加出站策略（单次 state 保存 + 单次 dataplane apply）。
func (srv *Server) handleNetworkEgressPoliciesBulk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Policies     []store.EgressPolicy `json:"policies"`
		SkipExisting bool                 `json:"skip_existing"`
	}
	if err := readJSON(r, &body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
		return
	}
	if len(body.Policies) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policies[] required"})
		return
	}
	normalized := make([]store.EgressPolicy, 0, len(body.Policies))
	for i := range body.Policies {
		p := body.Policies[i]
		if err := store.NormalizeEgressPolicy(&p); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		st := srv.store.Get()
		if _, ok := store.FindWanLink(st.Network.WanLinks, p.WanLinkID); !ok {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "wan_link_id not found: " + p.WanLinkID})
			return
		}
		normalized = append(normalized, p)
	}
	var added, skipped int
	_ = srv.store.Update(func(st *store.State) {
		existingCIDR := map[string]struct{}{}
		for _, p := range st.Network.EgressPolicies {
			existingCIDR[p.CIDR] = struct{}{}
		}
		for _, p := range normalized {
			if _, dup := existingCIDR[p.CIDR]; dup {
				if body.SkipExisting {
					skipped++
					continue
				}
				continue
			}
			st.Network.EgressPolicies = append(st.Network.EgressPolicies, p)
			existingCIDR[p.CIDR] = struct{}{}
			added++
		}
		if added > 0 {
			store.SyncEgressRoutes(st)
		}
	})
	if added == 0 {
		msg := "no policies added"
		if skipped > 0 {
			msg = "all policies already exist"
		}
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	_ = srv.store.Save()
	if added > 0 {
		if err := srv.applyEgressDataPlane(); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	srv.auditLog(r, "network.egress.bulk", fmt.Sprintf("added=%d skipped=%d", added, skipped))
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "added": added, "skipped": skipped})
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
	if w, ok := store.FindWanLink(st.Network.WanLinks, wanID); ok && store.IsWarpWanLink(w) {
		return errWarpWanLinkLocked{}
	}
	if len(egressPoliciesUsingWanLink(st, wanID)) > 0 {
		return errWanLinkInUse{wanID: wanID}
	}
	return nil
}

func validateWanLinkMutable(st store.State, wanID string) error {
	if w, ok := store.FindWanLink(st.Network.WanLinks, wanID); ok && store.IsWarpWanLink(w) {
		return errWarpWanLinkLocked{}
	}
	return nil
}

type errWarpWanLinkLocked struct{}

func (errWarpWanLinkLocked) Error() string {
	return "WARP managed WAN link cannot be modified manually; disconnect WARP to remove"
}

type errWanLinkInUse struct{ wanID string }

func (e errWanLinkInUse) Error() string {
	return "wan link in use by egress policy: " + e.wanID
}
