package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleShaperTC(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"enabled":     st.Shaper.Enabled,
			"root_qdisc":  "fq",
			"fq_flows":    st.Shaper.FQFlows,
			"fq_quantum":  st.Shaper.FQQuantum,
		})
	case http.MethodPut:
		var body struct {
			FQFlows   int  `json:"fq_flows"`
			FQQuantum int  `json:"fq_quantum"`
			Apply     bool `json:"apply"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			if body.FQFlows >= 0 {
				st.Shaper.FQFlows = body.FQFlows
			}
			if body.FQQuantum >= 0 {
				st.Shaper.FQQuantum = body.FQQuantum
			}
		})
		if !srv.persistState(w) {
			return
		}
		if body.Apply && srv.setupComplete() && srv.shaperEnabled() {
			st := srv.store.Get()
			srv.applyShaperP0(st)
			srv.syncShaperDevices(st)
			srv.applyWGShapers(st)
			srv.setupOCServShaper(st)
		}
		srv.auditLog(r, "shaper.tc", "fq")
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"root_qdisc": "fq",
			"fq_flows":   st.Shaper.FQFlows,
			"fq_quantum": st.Shaper.FQQuantum,
		})
	default:
		writeMethodNotAllowed(w)
	}
}
