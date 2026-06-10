package api

import (
	"net/http"
	"strconv"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleShaperEnabled(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		writeJSON(w, http.StatusOK, map[string]any{"enabled": st.Shaper.Enabled})
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
			Apply   bool `json:"apply"`
		}
		if err := readJSON(r, &body); err != nil {
			writeBadJSON(w)
			return
		}
		_ = srv.store.Update(func(st *store.State) {
			st.Shaper.Enabled = body.Enabled
		})
		if !srv.persistState(w) {
			return
		}
		if body.Apply && srv.setupComplete() {
			if body.Enabled {
				srv.applyShaperRuntime()
			} else {
				srv.teardownShaperRuntime()
			}
		}
		srv.auditLog(r, "shaper.enabled", strconv.FormatBool(body.Enabled))
		writeJSON(w, http.StatusOK, map[string]any{
			"ok":      true,
			"enabled": body.Enabled,
		})
	default:
		writeMethodNotAllowed(w)
	}
}
