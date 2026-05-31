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
			"egress_policies":           policies,
			"wan_links":                 links,
			"resolved":                  resolved,
			"dev_wan":                   srv.env.DevWAN,
			"cloudflare_cdn_cidrs_ipv4": store.CloudflareCDNCIDRsV4(),
		})
	case http.MethodPost:
		var body store.EgressPolicy
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if err := store.NormalizeEgressPolicy(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		st := srv.store.Get()
		if _, ok := store.FindWanLink(st.Network.WanLinks, body.WanLinkID); !ok {
			writeBadRequest(w, "wan_link_id not found")
			return
		}
		for _, p := range st.Network.EgressPolicies {
			if p.CIDR == body.CIDR && p.ID != body.ID {
				writeBadRequest(w, "cidr already used by another egress policy")
				return
			}
		}
		if !srv.commitEgressChange(w, func(st *store.State) {
			st.Network.EgressPolicies = append(st.Network.EgressPolicies, body)
			store.SyncEgressRoutes(st)
		}) {
			return
		}
		srv.auditLog(r, "network.egress.add", body.ID)
		writeJSON(w, http.StatusOK, body)
	case http.MethodPut:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		var body store.EgressPolicy
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		body.ID = id
		if err := store.NormalizeEgressPolicy(&body); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		stBefore := srv.store.Get()
		if _, ok := store.FindWanLink(stBefore.Network.WanLinks, body.WanLinkID); !ok {
			writeBadRequest(w, "wan_link_id not found")
			return
		}
		for _, p := range stBefore.Network.EgressPolicies {
			if p.ID != id && p.CIDR == body.CIDR {
				writeBadRequest(w, "cidr already used")
				return
			}
		}
		var old store.EgressPolicy
		found := false
		backup := store.CloneEgressPolicies(stBefore.Network.EgressPolicies)
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
			writeNotFound(w, "egress policy not found")
			return
		}
		proposed := srv.store.Get()
		if err := srv.checkNftForState(proposed); err != nil {
			srv.setEgressPolicies(backup)
			writeNftApplyError(w, err)
			return
		}
		if old.ID != "" {
			policyroute.DeletePolicy(old, stBefore.Network.WanLinks)
		}
		if !srv.saveState(w) {
			srv.setEgressPolicies(backup)
			return
		}
		if err := srv.reloadNftAfterEgressRevert(backup); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "network.egress.put", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		var removed store.EgressPolicy
		var links []store.WanLink
		found := false
		backup := store.CloneEgressPolicies(srv.store.Get().Network.EgressPolicies)
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
			writeNotFound(w, "egress policy not found")
			return
		}
		proposed := srv.store.Get()
		if err := srv.checkNftForState(proposed); err != nil {
			srv.setEgressPolicies(backup)
			writeNftApplyError(w, err)
			return
		}
		policyroute.DeletePolicy(removed, links)
		if !srv.saveState(w) {
			srv.setEgressPolicies(backup)
			return
		}
		if err := srv.reloadNftAfterEgressRevert(backup); err != nil {
			writeApplyError(w, err)
			return
		}
		srv.auditLog(r, "network.egress.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) commitEgressChange(w http.ResponseWriter, mutate func(*store.State)) bool {
	st := srv.store.Get()
	backup := store.CloneEgressPolicies(st.Network.EgressPolicies)
	_ = srv.store.Update(mutate)
	proposed := srv.store.Get()
	if err := srv.checkNftForState(proposed); err != nil {
		srv.setEgressPolicies(backup)
		writeNftApplyError(w, err)
		return false
	}
	if !srv.saveState(w) {
		srv.setEgressPolicies(backup)
		return false
	}
	if err := srv.reloadNftAfterEgressRevert(backup); err != nil {
		writeApplyError(w, err)
		return false
	}
	return true
}

func (srv *Server) replayEgressOnBoot() {
	st := srv.store.Get()
	if len(st.Network.EgressPolicies) == 0 {
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("egress boot save: %v", err)
	}
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
		writeBadJSON(w)
		return
	}
	if len(body.Policies) == 0 {
		writeBadRequest(w, "policies[] required")
		return
	}
	normalized := make([]store.EgressPolicy, 0, len(body.Policies))
	for i := range body.Policies {
		p := body.Policies[i]
		if err := store.NormalizeEgressPolicy(&p); err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		st := srv.store.Get()
		if _, ok := store.FindWanLink(st.Network.WanLinks, p.WanLinkID); !ok {
			writeBadRequest(w, "wan_link_id not found: "+p.WanLinkID)
			return
		}
		normalized = append(normalized, p)
	}
	var added, skipped int
	if !srv.commitEgressChange(w, func(st *store.State) {
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
	}) {
		return
	}
	if added == 0 {
		msg := "no policies added"
		if skipped > 0 {
			msg = "all policies already exist"
		}
		writeBadRequest(w, msg)
		return
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
