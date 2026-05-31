package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) applyImportedStateDataplane() error {
	if err := srv.reloadNft(); err != nil {
		return fmt.Errorf("nft: %w", err)
	}
	st := srv.store.Get()
	if st.Nat.Nat64Enabled || st.Nat.Nptv6Enabled {
		if err := srv.applyNatStack(); err != nil {
			return fmt.Errorf("nat stack: %w", err)
		}
	}
	return nil
}

func (srv *Server) revertImportedState(w http.ResponseWriter, backup store.State, applyErr error) bool {
	srv.store.ReplaceState(backup)
	if !srv.persistState(w) {
		return false
	}
	if err := srv.reloadNft(); err != nil {
		writeApplyError(w, fmt.Errorf("import reverted on disk but nft reload failed: %v (original: %w)", err, applyErr))
		return false
	}
	if backup.Nat.Nat64Enabled || backup.Nat.Nptv6Enabled {
		if err := srv.applyNatStackWithRollback(&natStackSnapshot{
			Nat:  store.CloneNatState(backup.Nat),
			DHCP: store.CloneDHCP(backup.DHCP),
		}); err != nil {
			writeApplyError(w, fmt.Errorf("import reverted on disk but nat stack restore failed: %v (original: %w)", err, applyErr))
			return false
		}
	}
	writeApplyError(w, fmt.Errorf("import apply failed, state reverted: %w", applyErr))
	return false
}

func (srv *Server) commitStateImport(w http.ResponseWriter, r *http.Request, imported store.State, auditLabel string) {
	st := srv.store.Get()
	if strings.TrimSpace(imported.AdminUser) == "" {
		imported.AdminUser = st.AdminUser
	}
	if imported.AdminPassHash == "" {
		imported.AdminPassHash = st.AdminPassHash
	}
	backup, err := store.CloneState(st)
	if err != nil {
		writeApplyError(w, err)
		return
	}
	srv.store.ReplaceState(imported)
	if !srv.persistState(w) {
		srv.store.ReplaceState(backup)
		return
	}
	if err := srv.applyImportedStateDataplane(); err != nil {
		srv.revertImportedState(w, backup, err)
		return
	}
	srv.auditLog(r, "system.state.import", auditLabel)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
