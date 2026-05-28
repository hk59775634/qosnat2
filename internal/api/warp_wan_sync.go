package api

import (
	"github.com/hk59775634/qosnat2/internal/store"
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
	return srv.applyWanLinkDataPlane()
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
