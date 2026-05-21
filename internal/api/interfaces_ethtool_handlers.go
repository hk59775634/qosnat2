package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
)

func (srv *Server) handleInterfacesEthtool(w http.ResponseWriter, r *http.Request) {
	dev := strings.TrimSpace(r.URL.Query().Get("device"))
	if dev == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "device query required"})
		return
	}
	switch r.Method {
	case http.MethodGet:
		info, err := netif.GetEthtool(dev)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, info)
	case http.MethodPut:
		var body struct {
			RxRing   int                      `json:"rx_ring"`
			TxRing   int                      `json:"tx_ring"`
			Offloads netif.OffloadSetRequest `json:"offloads"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		hasRing := body.RxRing > 0 || body.TxRing > 0
		hasOff := body.Offloads.GRO != "" || body.Offloads.GSO != "" ||
			body.Offloads.TXCSUM != "" || body.Offloads.RXCSUM != ""
		if !hasRing && !hasOff {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rx_ring, tx_ring or offloads required"})
			return
		}
		if hasRing {
			if err := netif.SetRing(dev, body.RxRing, body.TxRing); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			srv.auditLog(r, "iface.ring", dev)
		}
		if hasOff {
			if err := netif.SetOffloads(dev, body.Offloads); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			srv.auditLog(r, "iface.offload", dev)
		}
		info, _ := netif.GetEthtool(dev)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "ethtool": info})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
