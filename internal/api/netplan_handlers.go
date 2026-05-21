package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) applyNetplan() error {
	st := srv.store.Get()
	if err := netif.ApplyNetplan(st.Network); err != nil {
		return err
	}
	// 同步 VLAN 逻辑名
	_ = srv.store.Update(func(st *store.State) {
		for i := range st.Network.VLANs {
			v := &st.Network.VLANs[i]
			if v.Name == "" && v.Parent != "" && v.VID > 0 {
				v.Name = netif.VLANName(v.Parent, v.VID)
			}
		}
	})
	_ = srv.store.Save()
	return nil
}

func (srv *Server) handleNetworkNetplanApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := srv.applyNetplan(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	srv.auditLog(r, "network.netplan.apply", "")
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) handleNetworkNetplan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	body, _, err := netif.RenderNetplan(st.Network)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"path":     netif.NetplanConfigPathForAPI(),
		"rendered": string(body),
		"ifaces":   st.Network.Ifaces,
		"vlans":    st.Network.VLANs,
	})
}
