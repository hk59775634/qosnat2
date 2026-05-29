package api

import (
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
	// WARP 在 netns 内维护 cloudflare-warp nft 表；reloadNft(flush ruleset) 会破坏隧道并损坏 veth peer。
	if err := policyroute.Apply(srv.store.Get()); err != nil {
		return err
	}
	warpnetns.ReconcileHostNAT()
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
	st := srv.store.Get()
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
	if warpnetns.IsConnected() {
		return
	}
	_ = srv.removeWarpWanLink()
}
