package api

import (
	"context"
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/singbox"
)

const proxyWatchdogInterval = 25 * time.Second

func (srv *Server) startProxyEgressWatchdog() {
	if srv.proxyWatchCancel != nil {
		return
	}
	clearStaleProxyTaskOnBoot()
	ctx, cancel := context.WithCancel(context.Background())
	srv.proxyWatchCancel = cancel
	go func() {
		ticker := time.NewTicker(proxyWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.proxyEgressWatchdogTick()
			}
		}
	}()
}

func (srv *Server) proxyEgressWatchdogTick() {
	if !singbox.Installed() {
		return
	}
	proxyTaskMu.Lock()
	busy := proxyTaskBusy
	proxyTaskMu.Unlock()
	if busy {
		return
	}
	for _, p := range srv.store.Get().Network.ProxyEgress {
		if !p.Enabled {
			continue
		}
		if singbox.IsRunning(p) {
			continue
		}
		log.Printf("proxy-egress watchdog: restarting %s", p.ID)
		if err := singbox.Start(p); err != nil {
			log.Printf("proxy-egress watchdog start %s: %v", p.ID, err)
			continue
		}
		if err := srv.applyProxyWanAfterStart(p); err != nil {
			log.Printf("proxy-egress watchdog apply %s: %v", p.ID, err)
		}
	}
}
