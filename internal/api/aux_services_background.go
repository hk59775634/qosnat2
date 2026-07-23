package api

import (
	"context"
	"time"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/jool"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/unbound"
)

const (
	auxServicesInterval = 45 * time.Second
	auxServicesInitial  = 5 * time.Second
)

func (srv *Server) startAuxServicesBackground(ctx context.Context) {
	go func() {
		first := true
		select {
		case <-ctx.Done():
			return
		case <-time.After(auxServicesInitial):
		}
		for {
			if first || srv.auxServicesDrift() {
				srv.applyAuxNatStack()
				first = false
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(auxServicesInterval):
			}
		}
	}()
}

func (srv *Server) auxServicesDrift() bool {
	if !srv.setupComplete() {
		return false
	}
	st := srv.store.Get()
	if st.Nat.Nat64Enabled {
		if !jool.Active() {
			return true
		}
		if st.Nat.DNS64.Mode == store.DNS64ModeLocal && !unbound.Active() {
			return true
		}
	}
	if st.DHCP.ServiceActive() || st.Nat.DNS64UsesDnsmasqRelay() {
		if !dnsmasq.ShowStatus().Active {
			return true
		}
	}
	return false
}
