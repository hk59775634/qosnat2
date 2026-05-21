package api

import (
	"net/http"

	"github.com/hk59775634/qosnat2/internal/audit"
)

func (srv *Server) auditUser(r *http.Request) string {
	st := srv.store.Get()
	if st.AdminUser != "" {
		return st.AdminUser
	}
	return srv.env.AdminUser
}

func (srv *Server) auditLog(r *http.Request, action, detail string) {
	audit.Log(srv.auditUser(r), action, detail)
}
