package api

import (
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/stats"
)

type dataplaneMetrics struct {
	mu sync.RWMutex

	nftReloadTotal     uint64
	nftReloadLastMS    float64
	nftReloadLastError string
	nftReloadLastAt    time.Time

	natStackTotal     uint64
	natStackLastMS    float64
	natStackLastError string
	natStackLastAt    time.Time
}

func (m *dataplaneMetrics) recordNftReload(d time.Duration, err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nftReloadTotal++
	m.nftReloadLastMS = float64(d.Milliseconds())
	m.nftReloadLastAt = time.Now().UTC()
	if err != nil {
		m.nftReloadLastError = err.Error()
	} else {
		m.nftReloadLastError = ""
	}
}

func (m *dataplaneMetrics) recordNatStack(d time.Duration, err error) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.natStackTotal++
	m.natStackLastMS = float64(d.Milliseconds())
	m.natStackLastAt = time.Now().UTC()
	if err != nil {
		m.natStackLastError = err.Error()
	} else {
		m.natStackLastError = ""
	}
}

func (m *dataplaneMetrics) snapshot(conntrack stats.System) map[string]any {
	out := map[string]any{
		"conntrack_count": conntrack.Conntrack,
		"conntrack_max":   conntrackMax(),
		"cpu_percent":     conntrack.CPUPercent,
		"mem_percent":     conntrack.MemPercent,
	}
	if m == nil {
		return out
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	out["nft_reload"] = map[string]any{
		"total":      m.nftReloadTotal,
		"last_ms":    m.nftReloadLastMS,
		"last_error": m.nftReloadLastError,
		"last_at":    formatMetricTime(m.nftReloadLastAt),
	}
	out["nat_stack_apply"] = map[string]any{
		"total":      m.natStackTotal,
		"last_ms":    m.natStackLastMS,
		"last_error": m.natStackLastError,
		"last_at":    formatMetricTime(m.natStackLastAt),
	}
	return out
}

func formatMetricTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func conntrackMax() int {
	b, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_max")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

func (srv *Server) handleMetricsOps(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sys := srv.collector().System()
	writeJSON(w, http.StatusOK, srv.dataplaneMetrics.snapshot(sys))
}

func (srv *Server) handleMetricsPrometheus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	sys := srv.collector().System()
	snap := srv.dataplaneMetrics.snapshot(sys)
	nft, _ := snap["nft_reload"].(map[string]any)
	nat, _ := snap["nat_stack_apply"].(map[string]any)
	if nft == nil {
		nft = map[string]any{}
	}
	if nat == nil {
		nat = map[string]any{}
	}

	var b strings.Builder
	writePromGauge(&b, "qosnat_conntrack_count", float64(sys.Conntrack), "Current conntrack entries")
	writePromGauge(&b, "qosnat_conntrack_max", float64(conntrackMax()), "Conntrack table max")
	writePromGauge(&b, "qosnat_cpu_percent", sys.CPUPercent, "Host CPU utilization percent")
	writePromGauge(&b, "qosnat_mem_percent", sys.MemPercent, "Host memory utilization percent")
	writePromCounter(&b, "qosnat_nft_reload_total", toFloat(nft["total"]), "Total nft reload operations")
	writePromGauge(&b, "qosnat_nft_reload_last_ms", toFloat(nft["last_ms"]), "Last nft reload duration ms")
	writePromCounter(&b, "qosnat_nat_stack_apply_total", toFloat(nat["total"]), "Total NAT stack apply operations")
	writePromGauge(&b, "qosnat_nat_stack_apply_last_ms", toFloat(nat["last_ms"]), "Last NAT stack apply duration ms")

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(b.String()))
}

func writePromGauge(b *strings.Builder, name string, val float64, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s gauge\n%s %.6f\n\n", name, help, name, name, val)
}

func writePromCounter(b *strings.Builder, name string, val float64, help string) {
	fmt.Fprintf(b, "# HELP %s %s\n# TYPE %s counter\n%s %.0f\n\n", name, help, name, name, val)
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case uint64:
		return float64(n)
	default:
		return 0
	}
}
