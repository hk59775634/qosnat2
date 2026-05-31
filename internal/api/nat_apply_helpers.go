package api

import (
	"log"
	"net/http"

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
	return true
}

// commitNatStackChange 校验 nft → 持久化 → applyNatStack（含 jool/unbound/dnsmasq）。
func (srv *Server) commitNatStackChange(w http.ResponseWriter, mutate func(*store.State)) bool {
	st := srv.store.Get()
	backup := store.CloneNatState(st.Nat)
	_ = srv.store.Update(mutate)
	proposed := srv.store.Get()
	if err := srv.checkNftForState(proposed); err != nil {
		srv.setNatState(backup)
		writeNftApplyError(w, err)
		return false
	}
	if !srv.saveState(w) {
		srv.setNatState(backup)
		return false
	}
	if err := srv.applyNatStack(); err != nil {
		srv.setNatState(backup)
		_ = srv.store.Save()
		if revErr := srv.applyNatStack(); revErr != nil {
			log.Printf("revert nat stack after apply failure: %v", revErr)
		}
		writeApplyError(w, err)
		return false
	}
	return true
}
