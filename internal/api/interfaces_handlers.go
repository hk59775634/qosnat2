package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/stats"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleInterfaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleInterfacesGet(w, r)
	case http.MethodPut:
		srv.handleInterfacesPut(w, r)
	case http.MethodDelete:
		srv.handleInterfacesDelete(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleInterfacesGet(w http.ResponseWriter, r *http.Request) {
	list, err := netif.ListDetails()
	if err != nil {
		writeInternalError(w, err.Error())
		return
	}
	st := srv.store.Get()
	managedByDev := map[string]store.IfaceConfig{}
	for _, ic := range st.Network.Ifaces {
		managedByDev[ic.Device] = ic
	}
	c := srv.collector()
	type item struct {
		netif.Detail
		Role              string             `json:"role,omitempty"`
		Traffic           stats.IfaceRates   `json:"traffic"`
		Queues            int                `json:"rss_channels"`
		NetplanManageable bool               `json:"netplan_manageable"`
		Managed           *store.IfaceConfig `json:"managed,omitempty"`
	}
	out := make([]item, 0, len(list))
	for _, d := range list {
		role := ""
		if d.Name == srv.env.DevLAN {
			role = "LAN"
		} else if d.Name == srv.env.DevWAN {
			role = "WAN"
		}
		q := stats.IfaceQueues(d.Name)
		it := item{
			Detail:            d,
			Role:              role,
			Traffic:           c.IfaceMbps(d.Name),
			Queues:            q.Channels,
			NetplanManageable: ifaceNetplanManageable(d.Name, st),
		}
		if mc, ok := managedByDev[d.Name]; ok {
			cp := mc
			it.Managed = &cp
		}
		out = append(out, it)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dev_lan":         srv.env.DevLAN,
		"dev_wan":         srv.env.DevWAN,
		"netplan_path":    netif.NetplanConfigPathForAPI(),
		"interfaces":      out,
		"managed_ifaces":  st.Network.Ifaces,
		"traffic_history": c.TrafficHistory(),
	})
}

func (srv *Server) handleInterfacesPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Device string   `json:"device"`
		IPv4   []string `json:"ipv4"`
		Up     *bool    `json:"up"`
		DHCP4  *bool    `json:"dhcp4"`
	}
	if err := readJSON(r, &body); err != nil || strings.TrimSpace(body.Device) == "" {
		writeBadRequest(w, "device required")
		return
	}
	dev := strings.TrimSpace(body.Device)
	if err := netif.ValidateIfaceName(dev); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	if body.IPv4 == nil && body.Up == nil && body.DHCP4 == nil {
		writeBadRequest(w, "ipv4, up or dhcp4 required")
		return
	}
	st := srv.store.Get()
	if !ifaceNetplanManageable(dev, st) {
		writeBadRequest(w, "interface cannot be managed via netplan")
		return
	}
	if err := srv.applyNetplanWithRollback(func(st *store.State) error {
		store.UpsertIfaceConfig(st, dev, body.IPv4, body.Up, body.DHCP4)
		return nil
	}); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	srv.auditLog(r, "iface.netplan", dev)
	list, _ := netif.ListDetails()
	var updated *netif.Detail
	for i := range list {
		if list[i].Name == dev {
			updated = &list[i]
			break
		}
	}
	mc, _ := store.FindIfaceConfig(srv.store.Get(), dev)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"device":    dev,
		"interface": updated,
		"managed":   mc,
	})
}

func (srv *Server) handleInterfacesDelete(w http.ResponseWriter, r *http.Request) {
	dev := strings.TrimSpace(r.URL.Query().Get("device"))
	if dev == "" {
		writeBadRequest(w, "device required")
		return
	}
	if err := netif.ValidateIfaceName(dev); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	st := srv.store.Get()
	if _, ok := store.FindIfaceConfig(st, dev); !ok {
		writeNotFound(w, "managed iface not found")
		return
	}
	if err := srv.applyNetplanWithRollback(func(st *store.State) error {
		if !store.RemoveIfaceConfig(st, dev) {
			return fmt.Errorf("managed iface not found")
		}
		return nil
	}); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	srv.auditLog(r, "iface.netplan.clear", dev)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":     true,
		"device": dev,
	})
}

// ifaceNetplanManageable 物理口可写入 99-qosnat2 ethernets；VLAN/VXLAN/虚拟口走各自 API。
func ifaceNetplanManageable(dev string, st store.State) bool {
	if !netif.IsNetplanManagedDevice(dev) {
		return false
	}
	for _, v := range st.Network.VLANs {
		name := strings.TrimSpace(v.Name)
		if name == "" {
			name = netif.VLANName(v.Parent, v.VID)
		}
		if name == dev {
			return false
		}
	}
	for _, t := range st.Network.VXLANTunnels {
		name := strings.TrimSpace(t.Name)
		if name == "" {
			name = store.VXLANIfaceName(t.VNI)
		}
		if name == dev {
			return false
		}
	}
	for _, w := range st.Network.WanLinks {
		if store.IsManagedWanLink(w) && strings.TrimSpace(w.Device) == dev {
			return false
		}
	}
	return true
}

func (srv *Server) handleInterfacesRoles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		writeMethodNotAllowed(w)
		return
	}
	var body struct {
		DevLAN string `json:"dev_lan"`
		DevWAN string `json:"dev_wan"`
		Apply  *bool  `json:"apply"`
	}
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	body.DevLAN = strings.TrimSpace(body.DevLAN)
	body.DevWAN = strings.TrimSpace(body.DevWAN)
	if body.DevWAN == "" {
		writeBadRequest(w, "dev_wan required")
		return
	}
	if body.DevLAN != "" && body.DevLAN == body.DevWAN {
		writeBadRequest(w, "dev_lan and dev_wan must differ")
		return
	}
	if body.DevLAN != "" && !route.LinkExists(body.DevLAN) {
		writeBadRequest(w, fmt.Sprintf("dev_lan: interface %q not found", body.DevLAN))
		return
	}
	if !route.LinkExists(body.DevWAN) {
		writeBadRequest(w, fmt.Sprintf("dev_wan: interface %q not found", body.DevWAN))
		return
	}
	if err := WriteDevRoles(body.DevLAN, body.DevWAN); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	srv.reloadEnv()

	apply := true
	if body.Apply != nil {
		apply = *body.Apply
	}
	var applyErr string
	if srv.setupComplete() {
		if apply {
			if err := srv.ApplyAll(); err != nil {
				applyErr = err.Error()
			}
		} else if err := srv.reloadNft(); err != nil {
			applyErr = err.Error()
		}
	}
	srv.auditLog(r, "iface.roles", body.DevWAN+","+body.DevLAN)
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":          applyErr == "",
		"dev_lan":     body.DevLAN,
		"dev_wan":     body.DevWAN,
		"apply_error": applyErr,
	})
}
