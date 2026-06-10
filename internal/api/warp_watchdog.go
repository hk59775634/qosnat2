package api

import (
	"context"
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

const warpWatchdogInterval = 20 * time.Second

func (srv *Server) startWarpWatchdog() {
	if srv.warpWatchCancel != nil {
		return
	}
	clearStaleWarpTaskOnBoot()
	ctx, cancel := context.WithCancel(context.Background())
	srv.warpWatchCancel = cancel
	go func() {
		ticker := time.NewTicker(warpWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.warpWatchdogTick()
			}
		}
	}()
}

func (srv *Server) warpWatchdogTick() {
	if !srv.store.Get().Network.WarpEnabled {
		return
	}
	if !commandExists("warp-cli") {
		return
	}
	if !warpnetns.OpActive() && warpnetns.NeedsReset() && !warpnetns.IsConnected() {
		warpnetns.RepairStaleNetnsIfNeeded()
		warpnetns.ResetBroken()
	}
	warpnetns.EnsureHostNATOnly()
	if warpnetns.IsConnected() {
		warpnetns.RefreshConnectedState()
		srv.syncWarpStoreWhenEnabled()
		return
	}
	if !warpnetns.OpActive() && warpnetns.NetnsExists() && warpnetns.ServiceRunning() {
		_ = warpnetns.TryRepairConnectedNetns()
		if warpnetns.IsConnected() {
			iface := warpHostIface()
			if err := srv.applyWarpWanLink(iface); err != nil {
				log.Printf("warp watchdog repair apply: %v", err)
			}
			return
		}
	}
	srv.ensureWarpTunnelAsync("watchdog")
}

func warpHostIface() string {
	iface := warpnetns.HostInterface()
	if iface == "" {
		iface = "qwp0"
	}
	return iface
}

func (srv *Server) syncWarpStoreWhenEnabled() {
	if warpnetns.OpActive() {
		return
	}
	st := srv.store.Get()
	if !st.Network.WarpEnabled {
		return
	}
	iface := warpHostIface()
	_ = srv.store.Update(func(st *store.State) {
		store.UpsertWarpWanLink(st, iface)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("warp store sync: %v", err)
	}
}

func (srv *Server) ensureWarpTunnelAsync(reason string) {
	if !srv.store.Get().Network.WarpEnabled {
		return
	}
	if !commandExists("warp-cli") {
		return
	}
	if warpnetns.OpActive() {
		return
	}
	st := getWarpTaskStatus()
	if st.State == warpInstallStateRunning {
		return
	}
	// 刚失败不久时留给用户/UI 一次干净重试，避免与手动「启用」并发拆 netns。
	if st.State == warpInstallStateFailed && st.FinishedAt != "" {
		if t, err := time.Parse(time.RFC3339, st.FinishedAt); err == nil && time.Since(t) < 45*time.Second {
			return
		}
	}
	if err := srv.startWarpConnectAsync(nil); err != nil {
		if reason != "" {
			log.Printf("warp ensure (%s): %v", reason, err)
		}
	}
}
