package api

import (
	"log"
	"fmt"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/policyroute"
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
	vp := nft.VPNFirewallFromState(st)
	wanDevs := store.CollectWanInputDevices(srv.env.DevWAN, srv.env.DevLAN, st)
	rules, _ := store.SyncAutoFilterRules(
		st.Firewall.FilterRules,
		wanDevs,
		srv.env.AdminPort,
		store.AutoInputVPN{
			OCServEnabled: vp.OCServEnabled,
			OCServTCP:     vp.OCServTCP,
			OCServUDP:     vp.OCServUDP,
			WGPorts:       vp.WGPorts,
		},
		st.Firewall.WanPortForwards,
		srv.env.DevLAN,
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
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			_ = srv.reloadNftLocked()
			return err
		}
		return nil
	})
}

func (srv *Server) reloadNftWithForwardRevert(backupFwd []store.WanPortForward, backupRules []store.FilterRule) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setWanForwards(backupFwd)
			srv.setFilterRules(backupRules)
			srv.syncAutoFirewallRules()
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			_ = srv.reloadNftLocked()
			return err
		}
		return nil
	})
}

func (srv *Server) reloadNftWithAliasRevert(backup []store.AliasSet) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setAliases(backup)
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			_ = srv.reloadNftLocked()
			return err
		}
		return nil
	})
}

func writeNftApplyError(w http.ResponseWriter, err error) {
	writeAPIError(w, http.StatusUnprocessableEntity, "FIREWALL_NFT_INVALID",
		fmt.Sprintf("nft ruleset invalid: %v", err))
}

func writeSaveError(w http.ResponseWriter, err error) {
	writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "save failed: " + err.Error()})
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
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			_ = srv.reloadNftLocked()
			return err
		}
		return nil
	})
}

func (srv *Server) reloadNftWithNatIPv4Revert(backup store.NatIPv4State) error {
	return srv.withNftApply(func() error {
		if err := srv.reloadNftLocked(); err != nil {
			srv.setNatIPv4(backup)
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			_ = srv.reloadNftLocked()
			return err
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
			if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
			_ = srv.applyEgressDataPlaneLocked()
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
	if err := policyroute.Apply(srv.store.Get()); err != nil {
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

func firewallNftLines(rules []store.FilterRule) map[string]string {
	out := make(map[string]string, len(rules))
	for _, r := range rules {
		if line := r.NftRuleLine(); line != "" {
			out[r.ID] = line
		}
	}
	return out
}
