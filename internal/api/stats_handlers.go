package api

import (
	"net/http"
	"sort"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/stats"
)

func (srv *Server) collector() *stats.Collector {
	if srv.metrics == nil {
		srv.metrics = stats.New()
	}
	return srv.metrics
}

func (srv *Server) handleStatsDashboard(w http.ResponseWriter, r *http.Request) {
	c := srv.collector()
	st := srv.store.Get()
	lan := c.IfaceMbps(srv.env.DevLAN)
	wan := c.IfaceMbps(srv.env.DevWAN)
	sys := c.System()

	var active []ebpf.ActiveEntry
	var activeN int
	if srv.bpf != nil && srv.bpf.Ready() {
		list, err := srv.bpf.ListActive()
		if err == nil {
			active = list
			activeN = len(list)
		}
	}
	top := topActive(active, 10)
	mark := nft.AuditMarkIsolation()

	writeJSON(w, http.StatusOK, map[string]any{
		"phase":           "P5",
		"active_hosts":    activeN,
		"top_hosts":       top,
		"lan":             lan,
		"wan":             wan,
		"traffic_history": c.TrafficHistory(),
		"system":          sys,
		"ebpf":            srv.bpfStatus(),
		"shaper": map[string]any{
			"enabled":       st.Shaper.Enabled,
			"policy_cidr":   st.Shaper.PolicyCIDR,
			"profile_rules": len(st.Shaper.Profiles),
			"fq_flows":      st.Shaper.FQFlows,
			"fq_quantum":    st.Shaper.FQQuantum,
		},
		"mark_policy": mark,
		"interfaces": map[string]any{
			"lan": stats.IfaceQueues(srv.env.DevLAN),
			"wan": stats.IfaceQueues(srv.env.DevWAN),
		},
	})
}

func (srv *Server) bpfStatus() any {
	if srv.bpf == nil || !srv.bpf.Ready() {
		return map[string]any{"loaded": false}
	}
	return srv.bpf.Status()
}

type topHost struct {
	IP        string  `json:"ip"`
	BytesDown uint64  `json:"bytes_down"`
	BytesUp   uint64  `json:"bytes_up"`
	DownMbps  float64 `json:"down_mbps"`
	UpMbps    float64 `json:"up_mbps"`
}

func topActive(list []ebpf.ActiveEntry, n int) []topHost {
	type scored struct {
		ebpf.ActiveEntry
		total uint64
	}
	var s []scored
	for _, a := range list {
		s = append(s, scored{a, a.ActivityDown + a.ActivityUp})
	}
	sort.Slice(s, func(i, j int) bool { return s[i].total > s[j].total })
	if len(s) > n {
		s = s[:n]
	}
	out := make([]topHost, 0, len(s))
	for _, e := range s {
		out = append(out, topHost{
			IP:        e.IP,
			BytesDown: e.ActivityDown,
			BytesUp:   e.ActivityUp,
			DownMbps:  float64(e.ActivityDown) * 8 / 1e6,
			UpMbps:    float64(e.ActivityUp) * 8 / 1e6,
		})
	}
	return out
}
