package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/stats"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleInterfaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.handleInterfacesGet(w, r)
	case http.MethodPut:
		srv.handleInterfacesPut(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleInterfacesGet(w http.ResponseWriter, r *http.Request) {
	list, err := netif.ListDetails()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c := srv.collector()
	type item struct {
		netif.Detail
		Role    string           `json:"role,omitempty"`
		Traffic stats.IfaceRates `json:"traffic"`
		Queues  int              `json:"rss_channels"`
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
		out = append(out, item{
			Detail:  d,
			Role:    role,
			Traffic: c.IfaceMbps(d.Name),
			Queues:  q.Channels,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"dev_lan":          srv.env.DevLAN,
		"dev_wan":          srv.env.DevWAN,
		"netplan_path":     netif.NetplanConfigPathForAPI(),
		"interfaces":       out,
		"traffic_history":  c.TrafficHistory(),
	})
}

func (srv *Server) handleInterfacesPut(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Device string   `json:"device"`
		IPv4   []string `json:"ipv4"`
		Up     *bool    `json:"up"`
	}
	if err := readJSON(r, &body); err != nil || strings.TrimSpace(body.Device) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device required"})
		return
	}
	dev := strings.TrimSpace(body.Device)
	if body.IPv4 == nil && body.Up == nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ipv4 or up required"})
		return
	}
	if !netif.IsNetplanManagedDevice(dev) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "interface cannot be managed via netplan"})
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		store.UpsertIfaceConfig(st, dev, body.IPv4, body.Up, nil)
	})
	_ = srv.store.Save()
	if err := srv.applyNetplan(); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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
	writeJSON(w, http.StatusOK, map[string]any{
		"ok":        true,
		"device":    dev,
		"interface": updated,
	})
}
