package api

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/singbox"
	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	proxyInstallStateIdle    = "idle"
	proxyInstallStateRunning = "running"
	proxyInstallStateOK      = "ok"
	proxyInstallStateFailed  = "failed"
)

type proxyInstallStatus struct {
	State      string `json:"state"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
	LogTail    string `json:"log_tail,omitempty"`
}

var (
	proxyInstallMu   sync.Mutex
	proxyInstallBusy bool
)

func (srv *Server) handleNetworkProxyEgress(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		list := make([]store.ProxyEgress, 0, len(st.Network.ProxyEgress))
		for _, p := range st.Network.ProxyEgress {
			list = append(list, store.ProxyEgressPublicView(p))
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"proxy_egress": list,
			"installed":    singbox.Installed(),
			"version":      singbox.VersionString(),
		})
	case http.MethodPost:
		srv.handleProxyEgressCreate(w, r)
	case http.MethodPut:
		srv.handleProxyEgressUpdate(w, r)
	case http.MethodDelete:
		srv.handleProxyEgressDelete(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleProxyEgressCreate(w http.ResponseWriter, r *http.Request) {
	var body store.ProxyEgress
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	body.ID = ""
	body.TunIndex = -1
	body.EgressIP = ""
	if err := store.NormalizeProxyEgress(&body); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	var created store.ProxyEgress
	var allocErr error
	_ = srv.store.Update(func(st *store.State) {
		idx, e := store.AllocateProxyTunIndex(st.Network.ProxyEgress)
		if e != nil {
			allocErr = e
			return
		}
		body.TunIndex = idx
		st.Network.ProxyEgress = append(st.Network.ProxyEgress, body)
		store.UpsertProxyWanLink(st, body)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
		created = body
	})
	if allocErr != nil {
		writeBadRequest(w, allocErr.Error())
		return
	}
	if !srv.persistState(w) {
		return
	}
	srv.applyManagedRoutes()
	_ = srv.applyWanLinkDataPlane()
	srv.auditLog(r, "network.proxy_egress.add", created.ID)
	view := store.ProxyEgressPublicView(created)
	if singbox.Installed() {
		if startProxyTaskAsync(srv, "connect", created) {
			writeJSON(w, http.StatusAccepted, map[string]any{
				"ok":         true,
				"auto_test":  true,
				"proxy":      view,
				"task":       getProxyTaskStatus(),
			})
			return
		}
	}
	writeJSON(w, http.StatusOK, view)
}

func (srv *Server) handleProxyEgressUpdate(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeBadRequest(w, "id required")
		return
	}
	var body store.ProxyEgress
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	body.ID = id
	found := false
	wasEnabled := false
	var normErr error
	_ = srv.store.Update(func(st *store.State) {
		for i := range st.Network.ProxyEgress {
			if st.Network.ProxyEgress[i].ID != id {
				continue
			}
			found = true
			cur := st.Network.ProxyEgress[i]
			wasEnabled = cur.Enabled
			body.TunIndex = cur.TunIndex
			if body.Password == "" || body.Password == "***" {
				body.Password = cur.Password
			}
			body.EgressIP = cur.EgressIP
			if e := store.NormalizeProxyEgress(&body); e != nil {
				normErr = e
				return
			}
			// 启用意图由 connect/disconnect 控制，PUT 保留当前 Enabled。
			body.Enabled = cur.Enabled
			st.Network.ProxyEgress[i] = body
			store.UpsertProxyWanLink(st, body)
			store.SyncWanRoutes(st)
			store.SyncEgressRoutes(st)
			return
		}
	})
	if normErr != nil {
		writeBadRequest(w, normErr.Error())
		return
	}
	if !found {
		writeNotFound(w, "proxy egress not found")
		return
	}
	if !srv.persistState(w) {
		return
	}
	if wasEnabled {
		if p, ok := store.ProxyEgressByID(srv.store.Get().Network.ProxyEgress, id); ok {
			_ = singbox.Stop(id)
			if err := singbox.Start(p); err != nil {
				writeInternalError(w, err.Error())
				return
			}
			_ = srv.applyProxyWanAfterStart(p, singbox.ExitInfo{})
		}
	} else {
		srv.applyManagedRoutes()
		_ = srv.applyWanLinkDataPlane()
	}
	srv.auditLog(r, "network.proxy_egress.put", id)
	p, _ := store.ProxyEgressByID(srv.store.Get().Network.ProxyEgress, id)
	writeJSON(w, http.StatusOK, store.ProxyEgressPublicView(p))
}

func (srv *Server) handleProxyEgressDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeBadRequest(w, "id required")
		return
	}
	wanID := store.ProxyWanLinkID(id)
	st := srv.store.Get()
	if len(egressPoliciesUsingWanLink(st, wanID)) > 0 {
		writeBadRequest(w, "proxy egress in use by egress policy: "+wanID)
		return
	}
	_ = singbox.Stop(id)
	found := false
	_ = srv.store.Update(func(st *store.State) {
		var out []store.ProxyEgress
		for _, p := range st.Network.ProxyEgress {
			if p.ID == id {
				found = true
				continue
			}
			out = append(out, p)
		}
		st.Network.ProxyEgress = out
		store.RemoveProxyWanLink(st, id)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if !found {
		writeNotFound(w, "proxy egress not found")
		return
	}
	if !srv.persistState(w) {
		return
	}
	srv.applyManagedRoutes()
	_ = srv.applyWanLinkDataPlane()
	srv.auditLog(r, "network.proxy_egress.delete", id)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (srv *Server) handleNetworkProxyEgressStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	items := make([]map[string]any, 0, len(st.Network.ProxyEgress))
	for _, p := range st.Network.ProxyEgress {
		item := map[string]any{
			"id":          p.ID,
			"name":        p.Name,
			"enabled":     p.Enabled,
			"running":     singbox.IsRunning(p),
			"device":      store.ProxyTunDevice(p.TunIndex),
			"wan_link_id": store.ProxyWanLinkID(p.ID),
			"egress_ip":   p.EgressIP,
			"type":        p.Type,
			"server":      p.Server,
			"port":        p.Port,
		}
		if info := store.ProxyExitInfoFromStore(p); info != nil {
			item["exit_info"] = info
			item["test_ok"] = p.EgressIP != "" && strings.TrimSpace(p.LastTestError) == ""
		}
		items = append(items, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"installed":   singbox.Installed(),
		"version":     singbox.VersionString(),
		"root":        os.Getuid() == 0,
		"install_job": getProxyInstallStatus(),
		"task":        getProxyTaskStatus(),
		"items":       items,
	})
}

func (srv *Server) handleNetworkProxyEgressInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeBadRequest(w, "root required")
		return
	}
	if singbox.Installed() {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "installed": true, "version": singbox.VersionString()})
		return
	}
	if !startProxyInstallAsync() {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "message": "install already running"})
		return
	}
	srv.auditLog(r, "network.proxy_egress.install", "")
	writeJSON(w, http.StatusAccepted, map[string]any{"ok": true, "state": proxyInstallStateRunning})
}

func (srv *Server) handleNetworkProxyEgressUninstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}
	if os.Getuid() != 0 {
		writeBadRequest(w, "root required")
		return
	}
	proxyInstallMu.Lock()
	busy := proxyInstallBusy
	proxyInstallMu.Unlock()
	if busy {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "message": "install already running"})
		return
	}
	proxyTaskMu.Lock()
	taskBusy := proxyTaskBusy
	proxyTaskMu.Unlock()
	if taskBusy {
		writeJSON(w, http.StatusConflict, map[string]any{"ok": false, "message": "proxy task running"})
		return
	}
	st := srv.store.Get()
	if err := singbox.Uninstall(st.Network.ProxyEgress); err != nil {
		writeInternalError(w, err.Error())
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		for i := range st.Network.ProxyEgress {
			st.Network.ProxyEgress[i].Enabled = false
		}
		store.SyncProxyWanLinks(st)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if !srv.persistState(w) {
		return
	}
	srv.applyManagedRoutes()
	_ = srv.applyWanLinkDataPlane()
	_ = writeProxyInstallStatus(proxyInstallStatus{
		State:      proxyInstallStateIdle,
		Message:    "uninstalled",
		FinishedAt: time.Now().UTC().Format(time.RFC3339),
	})
	srv.auditLog(r, "network.proxy_egress.uninstall", "")
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "installed": false})
}

func (srv *Server) handleNetworkProxyEgressInstallStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, getProxyInstallStatus())
}

func startProxyInstallAsync() bool {
	proxyInstallMu.Lock()
	defer proxyInstallMu.Unlock()
	if proxyInstallBusy {
		return false
	}
	proxyInstallBusy = true
	started := time.Now().UTC().Format(time.RFC3339)
	_ = writeProxyInstallStatus(proxyInstallStatus{
		State:     proxyInstallStateRunning,
		Message:   "installing sing-box",
		StartedAt: started,
	})
	go func() {
		defer func() {
			proxyInstallMu.Lock()
			proxyInstallBusy = false
			proxyInstallMu.Unlock()
		}()
		_ = os.MkdirAll("/var/lib/qosnat2", 0755)
		logF, err := os.OpenFile(singbox.InstallLogFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			_ = writeProxyInstallStatus(proxyInstallStatus{
				State:      proxyInstallStateFailed,
				Message:    err.Error(),
				StartedAt:  started,
				FinishedAt: time.Now().UTC().Format(time.RFC3339),
			})
			return
		}
		defer logF.Close()
		w := io.MultiWriter(logF)
		err = singbox.DownloadAndInstall(w)
		finished := time.Now().UTC().Format(time.RFC3339)
		tail := readFileTail(singbox.InstallLogFile, 4<<10)
		if err != nil {
			_ = writeProxyInstallStatus(proxyInstallStatus{
				State:      proxyInstallStateFailed,
				Message:    err.Error(),
				StartedAt:  started,
				FinishedAt: finished,
				LogTail:    tail,
			})
			return
		}
		_ = writeProxyInstallStatus(proxyInstallStatus{
			State:      proxyInstallStateOK,
			Message:    "installed " + singbox.VersionString(),
			StartedAt:  started,
			FinishedAt: finished,
			LogTail:    tail,
		})
	}()
	return true
}

func writeProxyInstallStatus(st proxyInstallStatus) error {
	b, _ := json.Marshal(st)
	return os.WriteFile(singbox.InstallStatusFile, b, 0644)
}

func getProxyInstallStatus() proxyInstallStatus {
	b, err := os.ReadFile(singbox.InstallStatusFile)
	if err != nil {
		return proxyInstallStatus{State: proxyInstallStateIdle}
	}
	var st proxyInstallStatus
	if json.Unmarshal(b, &st) != nil || st.State == "" {
		return proxyInstallStatus{State: proxyInstallStateIdle}
	}
	st.LogTail = readFileTail(singbox.InstallLogFile, 4<<10)
	return st
}

func readFileTail(path string, max int) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	if len(b) > max {
		b = b[len(b)-max:]
	}
	return string(b)
}

func (srv *Server) applyProxyWanAfterStart(p store.ProxyEgress, info singbox.ExitInfo) error {
	if strings.TrimSpace(info.FetchedAt) == "" {
		info = singbox.ProbeExitInfo(p)
	}
	_ = srv.store.Update(func(st *store.State) {
		store.SetProxyEgressEnabled(st, p.ID, true)
		store.SetProxyEgressExitInfo(st, p.ID, storeProxyExitInfoToStore(info))
		if cur, ok := store.ProxyEgressByID(st.Network.ProxyEgress, p.ID); ok {
			store.UpsertProxyWanLink(st, cur)
		}
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		return err
	}
	srv.applyManagedRoutes()
	return srv.applyWanLinkDataPlane()
}

func (srv *Server) applyProxyWanAfterStop(id string) error {
	_ = srv.store.Update(func(st *store.State) {
		store.SetProxyEgressEnabled(st, id, false)
		if cur, ok := store.ProxyEgressByID(st.Network.ProxyEgress, id); ok {
			store.UpsertProxyWanLink(st, cur)
		} else {
			store.RemoveProxyWanLink(st, id)
		}
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		return err
	}
	srv.applyManagedRoutes()
	return srv.applyWanLinkDataPlane()
}

func (srv *Server) replayProxyEgressOnBoot() {
	st := srv.store.Get()
	_ = srv.store.Update(func(st *store.State) {
		store.SyncProxyWanLinks(st)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	_ = srv.persistStateOrLog("replay proxy egress on boot")
	if !singbox.Installed() {
		return
	}
	for _, p := range st.Network.ProxyEgress {
		if !p.Enabled {
			continue
		}
		go func(pe store.ProxyEgress) {
			if err := singbox.Start(pe); err != nil {
				return
			}
			_ = srv.applyProxyWanAfterStart(pe, singbox.ExitInfo{})
		}(p)
	}
}
