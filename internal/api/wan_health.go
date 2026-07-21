package api

import (
	"context"
	"log"
	"net"
	"net/http"
	"os/exec"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

type wanHealthStatus struct {
	ID           string `json:"id"`
	OK           bool   `json:"ok"`
	FailCount    int    `json:"fail_count"`
	LatencyMS    int    `json:"latency_ms,omitempty"`
	LastError    string `json:"last_error,omitempty"`
	CheckedAt    string `json:"checked_at,omitempty"`
	MonitorAddr  string `json:"monitor_addr,omitempty"`
	Unhealthy    bool   `json:"unhealthy"`
}

func (srv *Server) handleWanHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	st := srv.store.Get()
	srv.wanHealthMu.RLock()
	defer srv.wanHealthMu.RUnlock()
	out := make([]wanHealthStatus, 0, len(st.Network.WanLinks))
	for _, link := range st.Network.WanLinks {
		if !link.MonitorEnabled {
			continue
		}
		if h, ok := srv.wanHealth[link.ID]; ok {
			out = append(out, h)
			continue
		}
		out = append(out, wanHealthStatus{
			ID:          link.ID,
			OK:          true,
			MonitorAddr: store.WanLinkMonitorAddr(link),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"health": out})
}

func (srv *Server) startWanHealthBackground(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.tickWanHealth()
			}
		}
	}()
}

func (srv *Server) tickWanHealth() {
	st := srv.store.Get()
	now := time.Now()
	changed := false
	for _, link := range st.Network.WanLinks {
		if !link.Enabled || !link.MonitorEnabled || store.IsManagedWanLink(link) {
			continue
		}
		addr := store.WanLinkMonitorAddr(link)
		if addr == "" {
			continue
		}
		interval := store.WanLinkMonitorInterval(link)
		srv.wanHealthMu.RLock()
		prev, has := srv.wanHealth[link.ID]
		srv.wanHealthMu.RUnlock()
		if has && prev.CheckedAt != "" {
			if t, err := time.Parse(time.RFC3339, prev.CheckedAt); err == nil && now.Sub(t) < time.Duration(interval)*time.Second {
				continue
			}
		}
		ok, latency, errMsg := pingOnce(addr, 2*time.Second)
		h := wanHealthStatus{
			ID:          link.ID,
			OK:          ok,
			LatencyMS:   latency,
			LastError:   errMsg,
			CheckedAt:   now.UTC().Format(time.RFC3339),
			MonitorAddr: addr,
		}
		threshold := store.WanLinkMonitorLossThreshold(link)
		srv.wanHealthMu.Lock()
		if has {
			h.FailCount = prev.FailCount
			h.Unhealthy = prev.Unhealthy
		}
		if ok {
			h.FailCount = 0
			if h.Unhealthy {
				h.Unhealthy = false
				changed = true
			}
		} else {
			h.FailCount++
			if !h.Unhealthy && h.FailCount >= threshold {
				h.Unhealthy = true
				changed = true
			}
		}
		if srv.wanHealth == nil {
			srv.wanHealth = map[string]wanHealthStatus{}
		}
		srv.wanHealth[link.ID] = h
		srv.wanHealthMu.Unlock()
	}
	if changed {
		srv.reapplyRoutesAfterWanHealth()
	}
}

func (srv *Server) unhealthyWanIDs() map[string]bool {
	srv.wanHealthMu.RLock()
	defer srv.wanHealthMu.RUnlock()
	out := map[string]bool{}
	for id, h := range srv.wanHealth {
		if h.Unhealthy {
			out[id] = true
		}
	}
	return out
}

func (srv *Server) reapplyRoutesAfterWanHealth() {
	unhealthy := srv.unhealthyWanIDs()
	st := srv.store.Get()
	filtered := store.FilterWanLinksForRouting(st.Network.WanLinks, unhealthy)
	// 若全部主用链路都不健康，保留原集合，避免无默认路由黑洞
	hasMain := false
	for _, w := range filtered {
		if w.Enabled && !store.WanLinkUsesPolicyTableOnly(w, filtered) {
			hasMain = true
			break
		}
	}
	tmp := st
	if hasMain {
		tmp.Network.WanLinks = filtered
	}
	store.SyncWanRoutes(&tmp)
	store.SyncEgressRoutes(&tmp)
	_ = srv.store.Update(func(s *store.State) {
		s.Routes = tmp.Routes
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("wan health save routes: %v", err)
	}
	if err := srv.applyWanLinkDataPlane(); err != nil {
		log.Printf("wan health apply dataplane: %v", err)
	} else {
		log.Printf("wan health failover applied (unhealthy=%d)", len(unhealthy))
	}
}

func pingOnce(addr string, timeout time.Duration) (ok bool, latencyMS int, errMsg string) {
	// 优先 ICMP ping；无权限时回退 TCP/53 或 TCP/443 连通性
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	start := time.Now()
	cmd := exec.CommandContext(ctx, "ping", "-c", "1", "-W", "1", addr)
	if err := cmd.Run(); err == nil {
		return true, int(time.Since(start).Milliseconds()), ""
	}
	// TCP fallback
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(addr, "443"))
	if err != nil {
		conn, err = d.DialContext(ctx, "tcp", net.JoinHostPort(addr, "53"))
	}
	if err != nil {
		return false, 0, err.Error()
	}
	_ = conn.Close()
	return true, int(time.Since(start).Milliseconds()), ""
}
