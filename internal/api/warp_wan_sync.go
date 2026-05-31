package api

import (
	"log"
	"fmt"

	"github.com/hk59775634/qosnat2/internal/policyroute"
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
	if err := policyroute.Apply(srv.store.Get()); err != nil {
		return err
	}
	// 写入 qwp0 出站 masquerade 等 nft 规则；加载后 ReconcileHostNAT 回补 netns NAT/bypass。
	if srv.setupComplete() {
		if err := srv.reloadNft(); err != nil {
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

// reconcileWarpStoreState 清除 state 中残留的 WARP WAN 链路（netns 已损坏或未连接时）。
func (srv *Server) reconcileWarpStoreState() {
	if warpnetns.OpActive() {
		return
	}
	st := srv.store.Get()
	hasWarp := false
	for _, w := range st.Network.WanLinks {
		if store.IsWarpWanLink(w) {
			hasWarp = true
			break
		}
	}
	if warpnetns.IsConnected() {
		iface := warpnetns.HostInterface()
		if iface == "" {
			iface = "qwp0"
		}
		_ = srv.store.Update(func(st *store.State) {
			store.UpsertWarpWanLink(st, iface)
			store.SyncWanRoutes(st)
			store.SyncEgressRoutes(st)
		})
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		return
	}
	if !hasWarp {
		return
	}
	// netns 与 warp-svc 仍在时勿因瞬时探测失败删除 WARP WAN（会触发 reloadNft 并加剧 netns 抖动）。
	if warpnetns.NetnsExists() && warpnetns.ServiceRunning() {
		iface := warpnetns.HostInterface()
		if iface == "" {
			iface = "qwp0"
		}
		_ = warpnetns.TryRepairConnectedNetns()
		_ = srv.store.Update(func(st *store.State) {
			store.UpsertWarpWanLink(st, iface)
			store.SyncWanRoutes(st)
			store.SyncEgressRoutes(st)
		})
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		return
	}
	_ = srv.removeWarpWanLink()
}

// rollbackFailedWarpConnect 策略应用失败或健康检查失败时回滚 WARP 数据面。
func (srv *Server) rollbackFailedWarpConnect() {
	_ = srv.removeWarpWanLink()
	warpnetns.ScrubAfterFailedConnect()
}
