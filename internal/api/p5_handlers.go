package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/stats"
)

func (srv *Server) handleMarkPolicy(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, nft.AuditMarkIsolation())
}

func (srv *Server) handleIfaceQueues(w http.ResponseWriter, r *http.Request) {
	dev := r.URL.Query().Get("dev")
	if dev == "" {
		dev = srv.env.DevLAN
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"queues":   stats.IfaceQueues(dev),
		"softirq":  stats.SoftirqNET(),
		"lan":      stats.IfaceQueues(srv.env.DevLAN),
		"wan":      stats.IfaceQueues(srv.env.DevWAN),
	})
}

func (srv *Server) handleEbpfPrograms(w http.ResponseWriter, r *http.Request) {
	if srv.bpf == nil || !srv.bpf.Ready() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ebpf not loaded"})
		return
	}
	writeJSON(w, http.StatusOK, srv.bpf.ListPrograms(srv.env.DevLAN))
}

func (srv *Server) handleEbpfReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if srv.bpf == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "no bpf"})
		return
	}
	if err := srv.bpf.Reload(srv.env.DevLAN); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	st := srv.store.Get()
	_ = srv.bpf.ReplayState(st)
	ebpf.ReplayHostClasses(st, srv.hosts)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
