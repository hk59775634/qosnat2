package api

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/singbox"
	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	proxyTaskStateIdle    = "idle"
	proxyTaskStateRunning = "running"
	proxyTaskStateOK      = "ok"
	proxyTaskStateFailed  = "failed"

	proxyTaskStatusFile = "/var/lib/qosnat2/proxy-egress-task.json"
)

type proxyTaskStatus struct {
	State      string `json:"state"`
	Op         string `json:"op,omitempty"`
	ProxyID    string `json:"proxy_id,omitempty"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

var (
	proxyTaskMu   sync.Mutex
	proxyTaskBusy bool
)

func (srv *Server) handleNetworkProxyEgressConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = strings.TrimSpace(body.ID)
	}
	if id == "" {
		writeBadRequest(w, "id required")
		return
	}
	if !singbox.Installed() {
		writeBadRequest(w, "sing-box not installed")
		return
	}
	p, ok := store.ProxyEgressByID(srv.store.Get().Network.ProxyEgress, id)
	if !ok {
		writeNotFound(w, "proxy egress not found")
		return
	}
	if !startProxyTaskAsync(srv, "connect", p) {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "message": "task already running"})
		return
	}
	srv.auditLog(r, "network.proxy_egress.connect", id)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "state": proxyTaskStateRunning, "proxy_id": id})
}

func (srv *Server) handleNetworkProxyEgressDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		var body struct {
			ID string `json:"id"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		id = strings.TrimSpace(body.ID)
	}
	if id == "" {
		writeBadRequest(w, "id required")
		return
	}
	p, ok := store.ProxyEgressByID(srv.store.Get().Network.ProxyEgress, id)
	if !ok {
		writeNotFound(w, "proxy egress not found")
		return
	}
	// 先清启用意图，阻止 watchdog 抢连。
	_ = srv.store.Update(func(st *store.State) {
		store.SetProxyEgressEnabled(st, id, false)
	})
	_ = srv.store.Save()
	if !startProxyTaskAsync(srv, "disconnect", p) {
		_ = singbox.Stop(id)
		_ = srv.applyProxyWanAfterStop(id)
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "proxy_id": id})
		return
	}
	srv.auditLog(r, "network.proxy_egress.disconnect", id)
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "state": proxyTaskStateRunning, "proxy_id": id})
}

func (srv *Server) handleNetworkProxyEgressTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, getProxyTaskStatus())
}

func startProxyTaskAsync(srv *Server, op string, p store.ProxyEgress) bool {
	proxyTaskMu.Lock()
	defer proxyTaskMu.Unlock()
	if proxyTaskBusy {
		return false
	}
	proxyTaskBusy = true
	started := time.Now().UTC().Format(time.RFC3339)
	_ = writeProxyTaskStatus(proxyTaskStatus{
		State:     proxyTaskStateRunning,
		Op:        op,
		ProxyID:   p.ID,
		Message:   op + " starting",
		StartedAt: started,
	})
	go func() {
		defer func() {
			proxyTaskMu.Lock()
			proxyTaskBusy = false
			proxyTaskMu.Unlock()
		}()
		var err error
		switch op {
		case "connect":
			err = runProxyConnect(srv, p)
		case "disconnect":
			err = runProxyDisconnect(srv, p.ID)
		default:
			err = errString("unknown op")
		}
		finished := time.Now().UTC().Format(time.RFC3339)
		if err != nil {
			_ = writeProxyTaskStatus(proxyTaskStatus{
				State:      proxyTaskStateFailed,
				Op:         op,
				ProxyID:    p.ID,
				Message:    err.Error(),
				StartedAt:  started,
				FinishedAt: finished,
			})
			return
		}
		_ = writeProxyTaskStatus(proxyTaskStatus{
			State:      proxyTaskStateOK,
			Op:         op,
			ProxyID:    p.ID,
			Message:    op + " ok",
			StartedAt:  started,
			FinishedAt: finished,
		})
	}()
	return true
}

func runProxyConnect(srv *Server, p store.ProxyEgress) error {
	_ = srv.store.Update(func(st *store.State) {
		store.SetProxyEgressEnabled(st, p.ID, true)
	})
	_ = srv.store.Save()
	cur, ok := store.ProxyEgressByID(srv.store.Get().Network.ProxyEgress, p.ID)
	if !ok {
		return errString("proxy not found")
	}
	if err := singbox.Start(cur); err != nil {
		_ = srv.store.Update(func(st *store.State) {
			store.SetProxyEgressEnabled(st, p.ID, false)
		})
		_ = srv.store.Save()
		return err
	}
	return srv.applyProxyWanAfterStart(cur)
}

func runProxyDisconnect(srv *Server, id string) error {
	_ = singbox.Stop(id)
	return srv.applyProxyWanAfterStop(id)
}

func writeProxyTaskStatus(st proxyTaskStatus) error {
	_ = os.MkdirAll("/var/lib/qosnat2", 0755)
	b, _ := json.Marshal(st)
	return os.WriteFile(proxyTaskStatusFile, b, 0644)
}

func getProxyTaskStatus() proxyTaskStatus {
	b, err := os.ReadFile(proxyTaskStatusFile)
	if err != nil {
		return proxyTaskStatus{State: proxyTaskStateIdle}
	}
	var st proxyTaskStatus
	if json.Unmarshal(b, &st) != nil || st.State == "" {
		return proxyTaskStatus{State: proxyTaskStateIdle}
	}
	return st
}

func clearStaleProxyTaskOnBoot() {
	st := getProxyTaskStatus()
	if st.State == proxyTaskStateRunning {
		st.State = proxyTaskStateFailed
		st.Message = "interrupted by restart"
		st.FinishedAt = time.Now().UTC().Format(time.RFC3339)
		_ = writeProxyTaskStatus(st)
	}
}

type errString string

func (e errString) Error() string { return string(e) }
