package api

import (
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/jool"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/unbound"
)

type natStackSnapshot struct {
	Nat  store.NatState
	DHCP store.DHCPState
}

type natStackProgress struct {
	nft     bool
	jool    bool
	unbound bool
	dnsmasq bool
}

func (srv *Server) lastNatStackSnapshot() natStackSnapshot {
	srv.lastNatStackMu.Lock()
	defer srv.lastNatStackMu.Unlock()
	if srv.lastNatStackOK {
		return natStackSnapshot{
			Nat:  store.CloneNatState(srv.lastNatStackNat),
			DHCP: store.CloneDHCP(srv.lastNatStackDHCP),
		}
	}
	return natStackSnapshot{}
}

func (srv *Server) recordNatStackSuccess(st store.State) {
	srv.lastNatStackMu.Lock()
	defer srv.lastNatStackMu.Unlock()
	srv.lastNatStackNat = store.CloneNatState(st.Nat)
	srv.lastNatStackDHCP = store.CloneDHCP(st.DHCP)
	srv.lastNatStackOK = true
}

func (srv *Server) applyNftForState(st store.State) error {
	st = srv.syncedFirewallState(st)
	start := time.Now()
	err := nft.Apply(srv.nftCfg(), st)
	srv.dataplaneMetrics.recordNftReload(time.Since(start), err)
	if err != nil {
		return err
	}
	srv.persistAutoFirewallRules()
	srv.reconcileWarpAfterNft()
	return nil
}

func (srv *Server) unboundOptsForDHCP(st store.State, dhcp store.DHCPState) unbound.RenderOpts {
	patched := st
	patched.DHCP = dhcp
	return srv.unboundOpts(patched)
}

func (srv *Server) dnsmasqOptsFor(st store.State, nat store.NatState, dhcp store.DHCPState) dnsmasq.ApplyOpts {
	patched := st
	patched.Nat = nat
	patched.DHCP = dhcp
	return srv.dnsmasqOpts(patched)
}

func (srv *Server) applyDNSMasqFor(st store.State, nat store.NatState, dhcp store.DHCPState) error {
	patched := st
	patched.Nat = nat
	patched.DHCP = dhcp
	return srv.applyDNSMasqNAT(patched)
}

func (srv *Server) rollbackNatStackDataplane(prev natStackSnapshot, prog natStackProgress) {
	st := srv.store.Get()
	if prog.dnsmasq {
		if err := srv.applyDNSMasqFor(st, prev.Nat, prev.DHCP); err != nil {
			log.Printf("nat stack rollback dnsmasq: %v", err)
		}
	}
	if prog.unbound {
		if err := unbound.Apply(prev.Nat, srv.unboundOptsForDHCP(st, prev.DHCP)); err != nil {
			log.Printf("nat stack rollback unbound: %v", err)
		}
	}
	if prog.jool {
		if err := jool.Apply(prev.Nat); err != nil {
			log.Printf("nat stack rollback jool: %v", err)
		}
	}
	if prog.nft {
		patched := st
		patched.Nat = prev.Nat
		if err := srv.applyNftForState(patched); err != nil {
			log.Printf("nat stack rollback nft: %v", err)
		}
	}
}
