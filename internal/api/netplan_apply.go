package api

import (
	"fmt"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleNetworkNetplan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	body, _, err := netif.RenderNetplan(st.Network)
	if err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":   netif.NetplanConfigPathForAPI(),
		"yaml":   string(body),
		"vlans":  len(st.Network.VLANs),
		"ifaces": len(st.Network.Ifaces),
		"note":   "与 50-cloud-init.yaml 合并；同名 ethernets 以 99-qosnat2 为准",
	})
}

func (srv *Server) handleNetworkNetplanApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if err := srv.applyNetplanWithRollback(nil); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	srv.auditLog(r, "network.netplan.apply", "ok")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// applyNetplanWithRollback 备份 netplan + state，apply 失败则回滚
func (srv *Server) applyNetplanWithRollback(beforeSave func(*store.State) error) error {
	prev, err := store.CloneState(srv.store.Get())
	if err != nil {
		return err
	}
	npBackup, err := netif.BackupNetplanConfig()
	if err != nil {
		return err
	}
	if beforeSave != nil {
		var saveErr error
		_ = srv.store.Update(func(st *store.State) {
			saveErr = beforeSave(st)
		})
		if saveErr != nil {
			return saveErr
		}
		if err := srv.store.Save(); err != nil {
			return fmt.Errorf("save state: %w", err)
		}
	}
	if _, err := srv.applyNetplan(); err != nil {
		srv.store.ReplaceState(prev)
		if saveErr := srv.store.Save(); saveErr != nil {
			return fmt.Errorf("netplan apply failed and revert save failed: %v (apply: %w)", saveErr, err)
		}
		_ = netif.RestoreNetplanConfig(npBackup)
		return err
	}
	if err := netif.ApplyVirtualIPs(srv.store.Get().Network); err != nil {
		return err
	}
	return nil
}
