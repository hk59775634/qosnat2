package api

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

const wgEndpointPinInterval = 20 * time.Second

func (srv *Server) startWireGuardEndpointPinBackground(ctx context.Context) {
	go func() {
		// 启动稍后立刻钉一次，避免刚起来就被漫游走偏
		srv.pinWireGuardEndpoints("boot")
		ticker := time.NewTicker(wgEndpointPinInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.pinWireGuardEndpoints("watch")
			}
		}
	}()
}

func (srv *Server) pinWireGuardEndpoints(reason string) {
	if !srv.setupComplete() {
		return
	}
	st := srv.store.Get()
	for _, inst := range st.VPN.WireGuards {
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if !inst.Enabled || !netif.LinkExists(iface) {
			continue
		}
		fixed, err := wg.PinConfiguredEndpoints(iface, inst.Peers)
		if err != nil {
			log.Printf("wg endpoint pin (%s %s): %v", reason, iface, err)
			continue
		}
		for _, pub := range fixed {
			name := pub
			for _, p := range inst.Peers {
				if p.PublicKey == pub {
					if n := strings.TrimSpace(p.Name); n != "" {
						name = n
					}
					break
				}
			}
			log.Printf("wg endpoint pin (%s): %s peer %s restored to configured endpoint", reason, iface, name)
		}
	}
}

func (srv *Server) syncWireGuardEndpointRoutes() {
	_ = srv.store.Update(func(st *store.State) {
		wg.SyncEndpointRoutes(st)
	})
}
