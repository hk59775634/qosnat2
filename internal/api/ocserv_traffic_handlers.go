package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/ocserv/usertraffic"
)

func (srv *Server) handleOCServUserTraffic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	username := strings.TrimSpace(r.URL.Query().Get("username"))
	if username == "" {
		writeBadRequest(w, "username required")
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "7d"
	}
	switch period {
	case "24h", "7d", "30d", "365d", "1y":
	default:
		period = "7d"
	}
	if period == "1y" {
		period = "365d"
	}

	resp := usertraffic.DefaultStore().Query(username, period)

	st := srv.store.Get().VPN.OCServ
	cfg := ocserv.OcctlFromState(st)
	if cfg.UseOcctl {
		if rows, err := cfg.ShowUser(username); err == nil && len(rows) > 0 {
			resp.Online = true
			resp.Current = rows[0]
		} else if users, err := cfg.ShowUsers(); err == nil {
			for _, u := range users {
				name := usertrafficFieldString(u, "Username", "username")
				if strings.EqualFold(name, username) {
					resp.Online = true
					resp.Current = u
					break
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func usertrafficFieldString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}
