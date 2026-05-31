package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
)

func (srv *Server) handleInterfacesEthtool(w http.ResponseWriter, r *http.Request) {
	dev := strings.TrimSpace(r.URL.Query().Get("device"))
	if dev == "" {
		writeBadRequest(w, "device query required")
		return
	}
	if err := netif.ValidateIfaceName(dev); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	switch r.Method {
	case http.MethodGet:
		info, err := netif.GetEthtool(dev)
		if err != nil {
			writeBadRequest(w, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, info)
	case http.MethodPut:
		var body struct {
			RxRing   int                     `json:"rx_ring"`
			TxRing   int                     `json:"tx_ring"`
			Offloads netif.OffloadSetRequest `json:"offloads"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		hasRing := body.RxRing > 0 || body.TxRing > 0
		hasOff := body.Offloads.GRO != "" || body.Offloads.GSO != "" ||
			body.Offloads.TXCSUM != "" || body.Offloads.RXCSUM != ""
		if !hasRing && !hasOff {
			writeBadRequest(w, "rx_ring, tx_ring or offloads required")
			return
		}
		if hasRing {
			if err := netif.SetRing(dev, body.RxRing, body.TxRing); err != nil {
				writeBadRequest(w, err.Error())
				return
			}
			srv.auditLog(r, "iface.ring", dev)
		}
		if hasOff {
			if err := netif.SetOffloads(dev, body.Offloads); err != nil {
				writeBadRequest(w, err.Error())
				return
			}
			srv.auditLog(r, "iface.offload", dev)
		}
		info, _ := netif.GetEthtool(dev)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ethtool": info})
	default:
		writeMethodNotAllowed(w)
	}
}
