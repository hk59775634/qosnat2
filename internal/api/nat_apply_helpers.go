package api

import (
	"log"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/conntrack"
	"github.com/hk59775634/qosnat2/internal/store"
)

// commitNatIPv4Change 校验 nft → 持久化 → reload；失败回滚 IPv4 NAT 配置。
func (srv *Server) commitNatIPv4Change(w http.ResponseWriter, mutate func(*store.State)) bool {
	st := srv.store.Get()
	backup := store.CloneNatIPv4(st.Nat.IPv4)
	_ = srv.store.Update(mutate)
	proposed := srv.store.Get()
	if err := srv.checkNftForState(proposed); err != nil {
		srv.setNatIPv4(backup)
		writeNftApplyError(w, err)
		return false
	}
	if !srv.saveState(w) {
		srv.setNatIPv4(backup)
		return false
	}
	if err := srv.reloadNftWithNatIPv4Revert(backup); err != nil {
		writeApplyError(w, err)
		return false
	}
	conntrack.FlushByCIDRs(natIPv4ConntrackCIDRs(backup, proposed.Nat.IPv4))
	return true
}

// commitNatStackChange 校验 nft → 持久化 → applyNatStack（含 jool/unbound/dnsmasq）。
func (srv *Server) commitNatStackChange(w http.ResponseWriter, mutate func(*store.State)) bool {
	st := srv.store.Get()
	backupNat := store.CloneNatState(st.Nat)
	backupDHCP := store.CloneDHCP(st.DHCP)
	rollbackSnap := natStackSnapshot{Nat: backupNat, DHCP: backupDHCP}
	_ = srv.store.Update(mutate)
	proposed := srv.store.Get()
	if err := srv.checkNftForState(proposed); err != nil {
		srv.setNatState(backupNat)
		writeNftApplyError(w, err)
		return false
	}
	if !srv.saveState(w) {
		srv.setNatState(backupNat)
		return false
	}
	if err := srv.applyNatStackWithRollback(&rollbackSnap); err != nil {
		srv.setNatState(backupNat)
		if !srv.persistState(w) {
			return false
		}
		if revErr := srv.applyNatStackWithRollback(&rollbackSnap); revErr != nil {
			log.Printf("revert nat stack after apply failure: %v", revErr)
		}
		writeApplyError(w, err)
		return false
	}
	return true
}

func natIPv4ConntrackCIDRs(oldN, newN store.NatIPv4State) []string {
	seen := map[string]struct{}{}
	var out []string
	add := func(c string) {
		if c == "" {
			return
		}
		if _, ok := seen[c]; ok {
			return
		}
		seen[c] = struct{}{}
		out = append(out, c)
	}
	oldR := map[string]struct{}{}
	for _, c := range oldN.PolicyRoutes {
		oldR[c] = struct{}{}
	}
	newR := map[string]struct{}{}
	for _, c := range newN.PolicyRoutes {
		newR[c] = struct{}{}
		if _, ok := oldR[c]; !ok {
			add(c)
		}
	}
	for _, c := range oldN.PolicyRoutes {
		if _, ok := newR[c]; !ok {
			add(c)
		}
	}
	for inner := range oldN.StaticMappings {
		if newN.StaticMappings[inner] != oldN.StaticMappings[inner] {
			add(inner)
		}
	}
	for inner := range newN.StaticMappings {
		if _, ok := oldN.StaticMappings[inner]; !ok {
			add(inner)
		}
	}
	for inner := range oldN.PrefixMappings {
		if newN.PrefixMappings[inner] != oldN.PrefixMappings[inner] {
			add(inner)
		}
	}
	for inner := range newN.PrefixMappings {
		if _, ok := oldN.PrefixMappings[inner]; !ok {
			add(inner)
		}
	}
	return out
}
