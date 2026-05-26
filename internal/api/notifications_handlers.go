package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) pushUINotification(level, title, message, link string) {
	level = strings.TrimSpace(level)
	if level == "" {
		level = "info"
	}
	n := store.UINotification{
		Level:   level,
		Title:   strings.TrimSpace(title),
		Message: strings.TrimSpace(message),
		Link:    strings.TrimSpace(link),
	}
	if n.Title == "" {
		return
	}
	_ = srv.store.Update(func(s *store.State) {
		s.Notifications = store.PrependNotification(s.Notifications, n)
	})
	_ = srv.store.Save()
}

func (srv *Server) handleNotifications(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		unread := 0
		for _, n := range st.Notifications {
			if !n.Read {
				unread++
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"notifications": st.Notifications,
			"unread":        unread,
		})
	case http.MethodPost:
		var body struct {
			DismissAll bool     `json:"dismiss_all"`
			IDs        []string `json:"ids"`
			MarkRead   bool     `json:"mark_read"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			if body.DismissAll {
				s.Notifications = nil
				return
			}
			if len(body.IDs) == 0 {
				return
			}
			want := map[string]struct{}{}
			for _, id := range body.IDs {
				want[strings.TrimSpace(id)] = struct{}{}
			}
			var out []store.UINotification
			for _, n := range s.Notifications {
				if _, ok := want[n.ID]; !ok {
					out = append(out, n)
					continue
				}
				if body.MarkRead {
					n.Read = true
					out = append(out, n)
				}
			}
			s.Notifications = out
		})
		_ = srv.store.Save()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
