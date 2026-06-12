package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/stats"
)

func (srv *Server) handleMarkPolicy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, nft.AuditMarkIsolation())
}

func (srv *Server) handleIfaceQueues(w http.ResponseWriter, r *http.Request) {
	dev := strings.TrimSpace(r.URL.Query().Get("dev"))
	if dev == "" {
		dev = srv.env.DevLAN
	}
	if err := netif.ValidateIfaceName(dev); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"queues":  stats.IfaceQueues(dev),
		"softirq": stats.SoftirqNET(),
		"lan":     stats.IfaceQueues(srv.env.DevLAN),
		"wan":     stats.IfaceQueues(srv.env.DevWAN),
	})
}

func (srv *Server) handleEbpfPrograms(w http.ResponseWriter, r *http.Request) {
	if srv.bpf == nil || !srv.bpf.Ready() {
		writeUnavailable(w, "", "ebpf not loaded")
		return
	}
	writeJSON(w, http.StatusOK, srv.bpf.ListPrograms(srv.env.DevLAN))
}

func (srv *Server) handleEbpfReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if srv.bpf == nil {
		writeUnavailable(w, "", "no bpf")
		return
	}
	if err := srv.bpf.Reload(srv.env.DevLAN); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	st := srv.store.Get()
	_ = srv.bpf.ReplayState(st)
	srv.syncShaperDevices(st)
	srv.rebuildShaperDataPlane()
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
