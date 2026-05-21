package api

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/hk59775634/qosnat2/internal/audit"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/sysctl"
	"github.com/hk59775634/qosnat2/internal/tuning"
)

type tuningResponse struct {
	Catalog          []sysctl.Entry    `json:"catalog"`
	AppCatalog       []tuning.AppParam `json:"app_catalog"`
	Defaults         map[string]string `json:"defaults"`
	Performance      map[string]string `json:"performance_preset"`
	Saved            map[string]string `json:"saved"`
	Effective        map[string]string `json:"effective"`
	Live             map[string]string `json:"live"`
	App              map[string]any    `json:"app"`
	Recommended      tuning.Result     `json:"recommended"`
	Hardware         tuning.HostInfo   `json:"hardware"`
	HardwareTier     string            `json:"hardware_tier"`
	HardwareTierLabel string           `json:"hardware_tier_label"`
	PerfPreset       bool              `json:"perf_preset"`
	TuningAutoApplied bool             `json:"tuning_auto_applied"`
	TuningTier       string            `json:"tuning_tier"`
	DevLAN           string            `json:"dev_lan"`
	DevWAN           string            `json:"dev_wan"`
	TxQueueLenLAN    int               `json:"txqueuelen_lan"`
	TxQueueLenWAN    int               `json:"txqueuelen_wan"`
	LiveTxQLenLAN    int               `json:"live_txqueuelen_lan"`
	LiveTxQLenWAN    int               `json:"live_txqueuelen_wan"`
	RpsLAN           bool              `json:"rps_lan"`
	RpsWAN           bool              `json:"rps_wan"`
	ConfPath         string            `json:"conf_path"`
}

type tuningPutBody struct {
	Sysctl              map[string]string `json:"sysctl"`
	App                 map[string]any    `json:"app"`
	PerfPreset          *bool             `json:"perf_preset"`
	TxQueueLenLAN       *int              `json:"txqueuelen_lan"`
	TxQueueLenWAN       *int              `json:"txqueuelen_wan"`
	RpsLAN              *bool             `json:"rps_lan"`
	RpsWAN              *bool             `json:"rps_wan"`
	Apply               bool              `json:"apply"`
	ApplyRecommended    bool              `json:"apply_recommended"`
}

func (srv *Server) handleSystemTuning(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		srv.getSystemTuning(w, r)
	case http.MethodPut:
		srv.putSystemTuning(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) getSystemTuning(w http.ResponseWriter, _ *http.Request) {
	if srv.setupComplete() {
		st := srv.store.Get()
		if !st.System.TuningAutoApplied {
			_ = srv.store.Update(func(st *store.State) {
				tuning.AutoApplyOnSetup(st)
			})
			_ = srv.store.Save()
		}
	}
	st := srv.store.Get()
	host := tuning.DetectHost()
	tier := tuning.ClassifyTier(host)
	rec := tuning.Recommend(host)
	keys := catalogKeys()
	writeJSON(w, http.StatusOK, tuningResponse{
		Catalog:           sysctl.Catalog,
		AppCatalog:        tuning.AppCatalog,
		Defaults:          sysctl.Defaults,
		Performance:       sysctl.PerformancePreset,
		Saved:             cloneMap(st.System.Sysctl),
		Effective:         sysctl.Merge(st.System.Sysctl, st.System.PerfPreset),
		Live:              sysctl.ReadLive(keys),
		App:               tuning.AppValues(st),
		Recommended:       rec,
		Hardware:          host,
		HardwareTier:      string(tier),
		HardwareTierLabel: tuning.TierLabel(tier),
		PerfPreset:        st.System.PerfPreset,
		TuningAutoApplied: st.System.TuningAutoApplied,
		TuningTier:        st.System.TuningTier,
		DevLAN:            srv.env.DevLAN,
		DevWAN:            srv.env.DevWAN,
		TxQueueLenLAN:     st.System.TxQueueLenLAN,
		TxQueueLenWAN:     st.System.TxQueueLenWAN,
		LiveTxQLenLAN:     liveTxQLen(srv.env.DevLAN),
		LiveTxQLenWAN:     liveTxQLen(srv.env.DevWAN),
		RpsLAN:            st.System.RpsLAN,
		RpsWAN:            st.System.RpsWAN,
		ConfPath:          "/etc/sysctl.d/99-qosnat2.conf",
	})
}

func (srv *Server) putSystemTuning(w http.ResponseWriter, r *http.Request) {
	var body tuningPutBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}
	var appliedRec *tuning.Result
	if err := srv.store.Update(func(st *store.State) {
		sys := st.System
		if body.ApplyRecommended {
			host := tuning.DetectHost()
			rec := tuning.Recommend(host)
			tuning.ApplyResult(st, rec, false)
			appliedRec = &rec
			return
		}
		if body.Sysctl != nil {
			if sys.Sysctl == nil {
				sys.Sysctl = map[string]string{}
			}
			for k, v := range body.Sysctl {
				if v == "" {
					delete(sys.Sysctl, k)
				} else {
					sys.Sysctl[k] = v
				}
			}
		}
		tuning.ApplyAppValues(st, body.App)
		if body.PerfPreset != nil {
			sys.PerfPreset = *body.PerfPreset
		}
		if body.TxQueueLenLAN != nil {
			sys.TxQueueLenLAN = *body.TxQueueLenLAN
		}
		if body.TxQueueLenWAN != nil {
			sys.TxQueueLenWAN = *body.TxQueueLenWAN
		}
		if body.RpsLAN != nil {
			sys.RpsLAN = *body.RpsLAN
		}
		if body.RpsWAN != nil {
			sys.RpsWAN = *body.RpsWAN
		}
		st.System = sys
	}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := map[string]any{"ok": true, "applied": false}
	if appliedRec != nil {
		resp["tier"] = appliedRec.Tier
		resp["memory_budget"] = appliedRec.MemoryBudget
	}
	if body.Apply {
		st := srv.store.Get()
		if err := srv.applySystemTuning(st); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		resp["applied"] = true
	}
	srv.auditLog(r, "system.tuning.put", audit.FormatDetail("apply=", body.Apply, "recommended=", body.ApplyRecommended))
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) applySystemTuning(st store.State) error {
	sys := st.System
	sysctl.ApplyFast(sys.Sysctl, sys.PerfPreset)
	if err := sysctl.Apply(sys.Sysctl, sys.PerfPreset); err != nil {
		return err
	}
	if srv.env.DevLAN != "" {
		qlen := netif.EffectiveTxQLen(sys.TxQueueLenLAN)
		if err := netif.SetTxQueueLen(srv.env.DevLAN, qlen); err != nil {
			log.Printf("txqueuelen %s: %v", srv.env.DevLAN, err)
		}
		if sys.RpsLAN {
			if err := netif.ApplyRPS(srv.env.DevLAN); err != nil {
				log.Printf("rps %s: %v", srv.env.DevLAN, err)
			}
		}
	}
	if srv.env.DevWAN != "" {
		qlen := netif.EffectiveTxQLen(sys.TxQueueLenWAN)
		if err := netif.SetTxQueueLen(srv.env.DevWAN, qlen); err != nil {
			log.Printf("txqueuelen %s: %v", srv.env.DevWAN, err)
		}
		if sys.RpsWAN {
			if err := netif.ApplyRPS(srv.env.DevWAN); err != nil {
				log.Printf("rps %s: %v", srv.env.DevWAN, err)
			}
		}
	}
	return nil
}

// applyAutoTuningOnSetup 引导完成时按 CPU/内存写入推荐值并可选立即应用
func (srv *Server) applyAutoTuningOnSetup(applyNow bool) tuning.Result {
	var rec tuning.Result
	_ = srv.store.Update(func(st *store.State) {
		rec = tuning.AutoApplyOnSetup(st)
	})
	if applyNow && srv.setupComplete() {
		st := srv.store.Get()
		if err := srv.applySystemTuning(st); err != nil {
			log.Printf("auto tuning apply: %v", err)
		}
	}
	return rec
}

func catalogKeys() []string {
	keys := make([]string, 0, len(sysctl.Catalog)+len(sysctl.Defaults))
	seen := map[string]bool{}
	for _, e := range sysctl.Catalog {
		if !seen[e.Key] {
			keys = append(keys, e.Key)
			seen[e.Key] = true
		}
	}
	for k := range sysctl.Defaults {
		if !seen[k] {
			keys = append(keys, k)
			seen[k] = true
		}
	}
	return keys
}

func liveTxQLen(dev string) int {
	if dev == "" {
		return 0
	}
	n, _ := netif.TxQueueLen(dev)
	return n
}

func cloneMap(m map[string]string) map[string]string {
	if m == nil {
		return map[string]string{}
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
