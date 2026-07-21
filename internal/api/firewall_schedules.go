package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleFirewallSchedules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		list := st.Firewall.Schedules
		if list == nil {
			list = []store.Schedule{}
		}
		writeJSON(w, http.StatusOK, map[string]any{"schedules": list})
	case http.MethodPost:
		srv.handleFirewallSchedulesUpsert(w, r)
	case http.MethodDelete:
		srv.handleFirewallSchedulesDelete(w, r)
	default:
		writeMethodNotAllowed(w)
	}
}

func (srv *Server) handleFirewallSchedulesUpsert(w http.ResponseWriter, r *http.Request) {
	var body store.Schedule
	if err := readJSON(r, &body); err != nil {
		writeBadJSON(w)
		return
	}
	if err := store.NormalizeSchedule(&body); err != nil {
		writeBadRequest(w, err.Error())
		return
	}
	_ = srv.store.Update(func(st *store.State) {
		found := false
		for i := range st.Firewall.Schedules {
			if st.Firewall.Schedules[i].ID == body.ID {
				st.Firewall.Schedules[i] = body
				found = true
				break
			}
		}
		if !found {
			st.Firewall.Schedules = append(st.Firewall.Schedules, body)
		}
	})
	if !srv.persistState(w) {
		return
	}
	// 时间表变更可能立即影响规则生效窗口
	_ = srv.reloadNft()
	srv.auditLog(r, "firewall.schedule.upsert", body.ID)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "schedule": body})
}

func (srv *Server) handleFirewallSchedulesDelete(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.URL.Query().Get("id"))
	if id == "" {
		writeBadRequest(w, "id query required")
		return
	}
	st := srv.store.Get()
	rules := append(srv.workingFilterRules(st), st.Firewall.FilterRules...)
	if store.ScheduleReferencedByRules(rules, id) {
		writeBadRequest(w, "schedule is referenced by firewall rules")
		return
	}
	removed := false
	_ = srv.store.Update(func(st *store.State) {
		keep := make([]store.Schedule, 0, len(st.Firewall.Schedules))
		for _, s := range st.Firewall.Schedules {
			if s.ID == id {
				removed = true
				continue
			}
			keep = append(keep, s)
		}
		st.Firewall.Schedules = keep
	})
	if !removed {
		writeNotFound(w, "schedule not found")
		return
	}
	if !srv.persistState(w) {
		return
	}
	_ = srv.reloadNft()
	srv.auditLog(r, "firewall.schedule.delete", id)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}
