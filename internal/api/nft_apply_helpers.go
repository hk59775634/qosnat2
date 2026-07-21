package api

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
)

func cloneFilterRules(rules []store.FilterRule) []store.FilterRule {
	return append([]store.FilterRule(nil), rules...)
}

func cloneWanForwards(fwd []store.WanPortForward) []store.WanPortForward {
	return append([]store.WanPortForward(nil), fwd...)
}

func cloneAliases(aliases []store.AliasSet) []store.AliasSet {
	return append([]store.AliasSet(nil), aliases...)
}

func (srv *Server) syncedFirewallState(st store.State) store.State {
	wanDevs := store.CollectWanInputDevices(srv.env.DevWAN, srv.env.DevLAN, st)
	rules, _ := store.SyncAutoFilterRules(
		st.Firewall.FilterRules,
		wanDevs,
		srv.env.AdminPort,
		nft.AutoInputFromState(st),
		st.Firewall.WanPortForwards,
		st.LVS,
		srv.env.DevLAN,
		srv.env.DevWAN,
		nft.HairpinAddrResolver(srv.env.DevLAN, srv.env.DevWAN),
	)
	st.Firewall.FilterRules = rules
	return st
}

func (srv *Server) checkNftForState(st store.State) error {
	st = srv.syncedFirewallState(st)
	body, err := nft.Render(srv.nftCfg(), st)
	if err != nil {
		return err
	}
	if err := nft.CheckRuleset(body); err != nil {
		if nftCheckSkipped(err) {
			return nil
		}
		return err
	}
	return nil
}

func nftCheckSkipped(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "permission denied")
}

func (srv *Server) setFilterRules(rules []store.FilterRule) {
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.FilterRules = rules
	})
}

func (srv *Server) setWanForwards(fwd []store.WanPortForward) {
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.WanPortForwards = fwd
	})
}

func (srv *Server) setAliases(aliases []store.AliasSet) {
	_ = srv.store.Update(func(st *store.State) {
		st.Firewall.Aliases = aliases
	})
}

func (srv *Server) reloadNftWithFilterRevert(backup []store.FilterRule) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setFilterRules(backup)
			if !srv.persistStateOrLog("revert filter rules") {
				log.Printf("revert filter rules: save failed")
			}
			return srv.revertReloadError("filter rules", err)
		}
		return nil
	})
}

func (srv *Server) revertReloadError(what string, applyErr error) error {
	if rerr := srv.reloadNftLocked(); rerr != nil {
		log.Printf("revert %s reload: %v", what, rerr)
		return fmt.Errorf("%s nft apply: %w; revert reload: %v", what, applyErr, rerr)
	}
	return applyErr
}

func (srv *Server) reloadNftWithForwardRevert(backupFwd []store.WanPortForward, backupRules []store.FilterRule) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setWanForwards(backupFwd)
			srv.setFilterRules(backupRules)
			srv.syncAutoFirewallRules()
			if !srv.persistStateOrLog("nft revert save") {
			}
			return srv.revertReloadError("forward rules", err)
		}
		return nil
	})
}

func (srv *Server) reloadNftWithAliasRevert(backup []store.AliasSet) error {
	return srv.withNftApply(func() error {
		// 别名 API 路径已自行刷新/写入 members，此处不再二次 DNS/URL 拉取。
		if err := srv.applyNftLocked(); err != nil {
			srv.setAliases(backup)
			if !srv.persistStateOrLog("nft revert save") {
			}
			_ = srv.applyEgressPolicyRoutes()
			return srv.revertReloadError("aliases", err)
		}
		return nil
	})
}

func writeNftApplyError(w http.ResponseWriter, err error) {
	writeAPIError(w, http.StatusUnprocessableEntity, "FIREWALL_NFT_INVALID",
		fmt.Sprintf("nft ruleset invalid: %v", err))
}

func writeSaveError(w http.ResponseWriter, err error) {
	writeAPIError(w, http.StatusInternalServerError, "SAVE_FAILED", "save failed: "+err.Error())
}

func (srv *Server) saveState(w http.ResponseWriter) bool {
	if err := srv.store.Save(); err != nil {
		writeSaveError(w, err)
		return false
	}
	return true
}

func (srv *Server) setNatState(nat store.NatState) {
	_ = srv.store.Update(func(st *store.State) {
		st.Nat = nat
	})
}

func (srv *Server) reloadNftWithNatRevert(backup store.NatState) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setNatState(backup)
			if !srv.persistStateOrLog("nft revert save") {
			}
			return srv.revertReloadError("nat", err)
		}
		return nil
	})
}

func (srv *Server) reloadNftWithNatIPv4Revert(backup store.NatIPv4State) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setNatIPv4(backup)
			if !srv.persistStateOrLog("nft revert save") {
			}
			return srv.revertReloadError("nat ipv4", err)
		}
		return nil
	})
}

func (srv *Server) setEgressPolicies(policies []store.EgressPolicy) {
	_ = srv.store.Update(func(st *store.State) {
		st.Network.EgressPolicies = policies
		store.SyncEgressRoutes(st)
	})
}

func (srv *Server) setNatIPv4(ipv4 store.NatIPv4State) {
	_ = srv.store.Update(func(st *store.State) {
		st.Nat.IPv4 = ipv4
	})
}

func (srv *Server) reloadNftAfterEgressRevert(backupPolicies []store.EgressPolicy) error {
	return srv.withNftApply(func() error {
		if err := srv.applyEgressDataPlaneLocked(); err != nil {
			srv.setEgressPolicies(backupPolicies)
			if !srv.persistStateOrLog("nft revert save") {
			}
			if rerr := srv.applyEgressDataPlaneLocked(); rerr != nil {
				log.Printf("revert egress dataplane: %v", rerr)
				return fmt.Errorf("egress dataplane: %w; revert: %v", err, rerr)
			}
			return err
		}
		return nil
	})
}

func (srv *Server) applyEgressDataPlaneLocked() error {
	if !srv.store.Get().SetupComplete {
		return nil
	}
	_ = srv.store.Update(func(st *store.State) {
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		return err
	}
	srv.applyManagedRoutes()
	if err := srv.applyEgressPolicyRoutes(); err != nil {
		return err
	}
	return srv.reloadNftLocked()
}

func (srv *Server) applyEgressDataPlane() error {
	return srv.withNftApply(func() error {
		return srv.applyEgressDataPlaneLocked()
	})
}

func (srv *Server) setNatStackStatus(status map[string]any) {
	srv.natStackStatusMu.Lock()
	defer srv.natStackStatusMu.Unlock()
	srv.natStackStatus = status
}

func (srv *Server) getNatStackStatus() map[string]any {
	srv.natStackStatusMu.RLock()
	defer srv.natStackStatusMu.RUnlock()
	if srv.natStackStatus == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(srv.natStackStatus))
	for k, v := range srv.natStackStatus {
		out[k] = v
	}
	return out
}

