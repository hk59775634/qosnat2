package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleNetworkVirtualIPs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleVirtualIPsGet(w, r)
	case http.MethodPost:
		srv.handleVirtualIPsPost(w, r)
	case http.MethodPut:
		srv.handleVirtualIPsPut(w, r)
	case http.MethodDelete:
		srv.handleVirtualIPsDelete(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleVirtualIPsGet(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	type item struct {
		store.VirtualIP
		Host     string `json:"host"`
		Assigned bool   `json:"assigned"`
	}
	list := st.Network.VirtualIPs
	if list == nil {
		list = []store.VirtualIP{}
	}
	out := make([]item, 0, len(list))
	for _, v := range list {
		host := store.VirtualIPHost(v)
		out = append(out, item{
			VirtualIP: v,
			Host:      host,
			Assigned:  netif.IsAssignedIP(host, v.Interface),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"virtual_ips":  out,
		"dev_wan":      srv.env.DevWAN,
		"dev_lan":      srv.env.DevLAN,
		"netplan_path": netif.NetplanConfigPathForAPI(),
	})
}

func (srv *Server) handleVirtualIPsPost(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Type      string `json:"type"`
		Interface string `json:"interface"`
		Address   string `json:"address"`
		Comment   string `json:"comment"`
		Enabled   *bool  `json:"enabled"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	vip := store.VirtualIP{
		Type:      body.Type,
		Interface: body.Interface,
		Address:   body.Address,
		Comment:   body.Comment,
		Enabled:   true,
	}
	if body.Enabled != nil {
		vip.Enabled = *body.Enabled
	}
	if err := store.NormalizeVirtualIP(&vip); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := netif.ValidateIfaceName(vip.Interface); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if !netif.LinkExists(vip.Interface) {
		writeBadRequest(w, "interface not found")
		return
	}
	if msg := virtualIPConflict(srv.store.Get(), vip, ""); msg != "" {
		writeBadRequest(w, msg)
		return
	}
	var saved store.VirtualIP
	if err := srv.applyVirtualIPChange(func(st *store.State) error {
		st.Network.VirtualIPs = append(st.Network.VirtualIPs, vip)
		saved = vip
		return nil
	}, "", ""); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	srv.auditLog(r, "virtual_ip.add", saved.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "virtual_ip": saved})
}

func (srv *Server) handleVirtualIPsPut(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeBadRequest(w, "id query required")
		return
	}
	var body struct {
		Type      string  `json:"type"`
		Interface string  `json:"interface"`
		Address   string  `json:"address"`
		Comment   *string `json:"comment"`
		Enabled   *bool   `json:"enabled"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	st := srv.store.Get()
	prev, ok := store.FindVirtualIP(st.Network.VirtualIPs, id)
	if !ok {
		writeNotFound(w, "virtual ip not found")
		return
	}
	vip := prev
	if body.Type != "" {
		vip.Type = body.Type
	}
	if body.Interface != "" {
		vip.Interface = body.Interface
	}
	if body.Address != "" {
		vip.Address = body.Address
	}
	if body.Comment != nil {
		vip.Comment = *body.Comment
	}
	if body.Enabled != nil {
		vip.Enabled = *body.Enabled
	}
	vip.ID = id
	if err := store.NormalizeVirtualIP(&vip); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if err := netif.ValidateIfaceName(vip.Interface); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if !netif.LinkExists(vip.Interface) {
		writeBadRequest(w, "interface not found")
		return
	}
	if msg := virtualIPConflict(st, vip, id); msg != "" {
		writeBadRequest(w, msg)
		return
	}
	found := false
	if err := srv.applyVirtualIPChange(func(st *store.State) error {
		for i := range st.Network.VirtualIPs {
			if st.Network.VirtualIPs[i].ID == id {
				st.Network.VirtualIPs[i] = vip
				found = true
				return nil
			}
		}
		return fmt.Errorf("virtual ip not found")
	}, prev.Interface, prev.Address); err != nil {
		if !found {
			writeNotFound(w, "virtual ip not found")
			return
		}
		writeBadRequest(w, err.Error())
		return
	}
	srv.auditLog(r, "virtual_ip.put", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "virtual_ip": vip})
}

func (srv *Server) handleVirtualIPsDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeBadRequest(w, "id query required")
		return
	}
	st := srv.store.Get()
	prev, ok := store.FindVirtualIP(st.Network.VirtualIPs, id)
	if !ok {
		writeNotFound(w, "virtual ip not found")
		return
	}
	if err := srv.applyVirtualIPChange(func(st *store.State) error {
		keep := make([]store.VirtualIP, 0, len(st.Network.VirtualIPs))
		found := false
		for _, v := range st.Network.VirtualIPs {
			if v.ID == id {
				found = true
				continue
			}
			keep = append(keep, v)
		}
		if !found {
			return fmt.Errorf("virtual ip not found")
		}
		st.Network.VirtualIPs = keep
		return nil
	}, prev.Interface, prev.Address); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	srv.auditLog(r, "virtual_ip.del", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// applyVirtualIPChange 更新 state → netplan → 绑定/解绑 alias；失败回滚。
func (srv *Server) applyVirtualIPChange(mutate func(*store.State) error, removeDev, removeCIDR string) error {
	before, err := store.CloneState(srv.store.Get())
	if err != nil {
		return err
	}
	npBackup, err := netif.BackupNetplanConfig()
	if err != nil {
		return err
	}
	var saveErr error
	_ = srv.store.Update(func(st *store.State) {
		saveErr = mutate(st)
	})
	if saveErr != nil {
		return saveErr
	}
	if err := srv.store.Save(); err != nil {
		srv.store.ReplaceState(before)
		return fmt.Errorf("save state: %w", err)
	}
	st := srv.store.Get()
	if removeDev != "" && removeCIDR != "" {
		host := store.VirtualIPHost(store.VirtualIP{Address: removeCIDR})
		if !virtualIPStillWanted(st, removeDev, host) {
			_ = netif.RemoveAddrFromDev(removeDev, removeCIDR)
		}
	}
	rollback := func(applyErr error) error {
		srv.store.ReplaceState(before)
		if saveErr := srv.store.Save(); saveErr != nil {
			return fmt.Errorf("virtual ip apply failed and revert save failed: %v (apply: %w)", saveErr, applyErr)
		}
		_ = netif.RestoreNetplanConfig(npBackup)
		_ = netif.ApplyVirtualIPs(before.Network)
		return applyErr
	}
	if _, err := srv.applyNetplan(); err != nil {
		return rollback(err)
	}
	if err := netif.ApplyVirtualIPs(st.Network); err != nil {
		return rollback(err)
	}
	return nil
}

func virtualIPStillWanted(st store.State, dev, host string) bool {
	dev = strings.TrimSpace(dev)
	host = strings.TrimSpace(host)
	for _, v := range st.Network.VirtualIPs {
		if !v.Enabled {
			continue
		}
		if strings.TrimSpace(v.Interface) == dev && store.VirtualIPHost(v) == host {
			return true
		}
	}
	for _, ic := range st.Network.Ifaces {
		if strings.TrimSpace(ic.Device) != dev {
			continue
		}
		for _, a := range ic.IPv4 {
			_, h, err := store.NormalizeVirtualIPAddress(a)
			if err == nil && h == host {
				return true
			}
		}
	}
	return false
}

func virtualIPConflict(st store.State, vip store.VirtualIP, skipID string) string {
	host := store.VirtualIPHost(vip)
	for _, v := range st.Network.VirtualIPs {
		if skipID != "" && v.ID == skipID {
			continue
		}
		if store.VirtualIPHost(v) == host {
			return "address already used by virtual ip " + v.ID
		}
	}
	return ""
}
