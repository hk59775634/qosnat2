package api

import (
	"fmt"
	"log"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

func (srv *Server) applyWarpWanLink(device string) error {
	_ = srv.store.Update(func(st *store.State) {
		store.UpsertWarpWanLink(st, device)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		return err
	}
	srv.applyManagedRoutes()
	if err := srv.applyEgressPolicyRoutes(); err != nil {
		return err
	}
	// 写入 qwp0 出站 masquerade 等 nft 规则；加载后 ReconcileHostNAT 回补 netns NAT/bypass。
	if srv.setupComplete() {
		// 连接任务进行中勿 flush 全表 nft（会破坏 netns/veth）；仅回补 WARP NAT/bypass。
		if warpnetns.OpActive() {
			warpnetns.EnsureHostNATOnly()
		} else if err := srv.reloadNft(); err != nil {
			return err
		}
	} else {
		warpnetns.EnsureHostNATOnly()
	}
	if warpnetns.IsConnected() {
		return warpnetns.ReconcileAfterWanLink()
	}
	return nil
}

// applyWarpPoliciesAfterConnect 在 WARP 隧道已稳定后应用 qosnat 策略（reload nft 后由 ReconcileHostNAT 回补 netns 规则）。
func (srv *Server) applyWarpPoliciesAfterConnect(iface string) error {
	if err := restoreRoutesAfterWarpConnect(srv); err != nil {
		return err
	}
	return srv.applyWarpWanLink(iface)
}

// verifyWarpTunnelHealthy 确认 netns 与 WARP 隧道在策略应用后仍可用。
func verifyWarpTunnelHealthy() error {
	if !warpnetns.NetnsHealthy() {
		return fmt.Errorf("warp netns unhealthy")
	}
	if !warpnetns.IsConnected() {
		return fmt.Errorf("warp tunnel not connected")
	}
	return nil
}

func (srv *Server) removeWarpWanLink() error {
	_ = srv.store.Update(func(st *store.State) {
		store.RemoveWarpWanLink(st)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		return err
	}
	srv.applyManagedRoutes()
	return srv.applyWanLinkDataPlane()
}

// reconcileWarpStoreState 按持久化 warp_enabled 同步 store，不因瞬时隧道探测失败删除 WAN/出站策略。
func (srv *Server) reconcileWarpStoreState() {
	if warpnetns.OpActive() {
		return
	}
	st := srv.store.Get()
	if !st.Network.WarpEnabled {
		hasWarp := false
		for _, w := range st.Network.WanLinks {
			if store.IsWarpWanLink(w) {
				hasWarp = true
				break
			}
		}
		if !hasWarp {
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			store.RemoveWarpWanLink(st)
			store.SyncWanRoutes(st)
			store.SyncEgressRoutes(st)
		})
		if err := srv.store.Save(); err != nil {
			log.Printf("reconcile warp store: save: %v", err)
		}
		return
	}
	iface := warpHostIface()
	if warpnetns.IsConnected() || (warpnetns.NetnsExists() && warpnetns.ServiceRunning()) {
		if !warpnetns.OpActive() && !warpnetns.IsConnected() {
			_ = warpnetns.TryRepairConnectedNetns()
		}
		if i := warpnetns.HostInterface(); i != "" {
			iface = i
		}
	}
	_ = srv.store.Update(func(st *store.State) {
		store.UpsertWarpWanLink(st, iface)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("reconcile warp store: save: %v", err)
	}
}

// rollbackFailedWarpConnect 策略应用失败或健康检查失败时回滚隧道，保留 warp_enabled 与出站策略供看门狗重试。
func (srv *Server) rollbackFailedWarpConnect() {
	warpnetns.ScrubAfterFailedConnect()
	srv.syncWarpStoreWhenEnabled()
}
