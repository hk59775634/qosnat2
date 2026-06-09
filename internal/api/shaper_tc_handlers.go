package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleShaperTC(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"leaf":       shaper.NormalizeLeaf(st.Shaper.Leaf),
			"fq_flows":   st.Shaper.FQFlows,
			"fq_quantum": st.Shaper.FQQuantum,
		})
	case http.MethodPut:
		var body struct {
			Leaf      string `json:"leaf"`
			FQFlows   int    `json:"fq_flows"`
			FQQuantum int    `json:"fq_quantum"`
			Apply     bool   `json:"apply"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		if !shaper.AllowedLeafInput(body.Leaf) {
			writeBadRequest(w, "invalid leaf (fq_codel or cake)")
			return
		}
		leaf := shaper.NormalizeLeaf(body.Leaf)
		_ = srv.store.Update(func(st *store.State) {
			st.Shaper.Leaf = leaf
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
		if body.Apply && srv.setupComplete() {
			st := srv.store.Get()
			if err := shaper.SetupP0(shaper.Config{
				DevLAN:     srv.env.DevLAN,
				Leaf:       st.Shaper.Leaf,
				FQFlows:    st.Shaper.FQFlows,
				FQQuantum:  st.Shaper.FQQuantum,
				TxQueueLen: st.System.TxQueueLenLAN,
			}); err != nil {
				writeInternalError(w, err.Error())
				return
			}
			srv.syncShaperDevices()
			srv.replayProfileHosts()
			srv.reattachShaperDataPath()
		}
		srv.auditLog(r, "shaper.tc", leaf)
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":         true,
			"leaf":       st.Shaper.Leaf,
			"fq_flows":   st.Shaper.FQFlows,
			"fq_quantum": st.Shaper.FQQuantum,
		})
	default:
		writeMethodNotAllowed(w)
	}
}
