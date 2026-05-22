package api

import (
	"strings"

	"golang.org/x/crypto/bcrypt"

	"github.com/hk59775634/qosnat2/internal/shaper"
)

func hashPassword(pass string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(h), nil
}

func checkPassword(hash, pass string) bool {
	if hash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pass)) == nil
}

func (srv *Server) setupComplete() bool {
	st := srv.store.Get()
	if st.SetupComplete {
		return true
	}
	return srv.env.DevLAN != "" && srv.env.DevWAN != ""
}

func (srv *Server) verifyAdmin(user, pass string) bool {
	role, ok := srv.verifyUserRole(user, pass)
	return ok && role == "admin"
}

func (srv *Server) verifyUserRole(user, pass string) (role string, ok bool) {
	st := srv.store.Get()
	user = strings.TrimSpace(user)
	if st.AdminPassHash != "" && st.AdminUser != "" && user == st.AdminUser {
		if checkPassword(st.AdminPassHash, pass) {
			return "admin", true
		}
		return "", false
	}
	if st.ReadOnlyPassHash != "" && st.ReadOnlyUser != "" && user == st.ReadOnlyUser {
		if checkPassword(st.ReadOnlyPassHash, pass) {
			return "readonly", true
		}
		return "", false
	}
	if st.AdminPassHash == "" && user == srv.env.AdminUser && pass == srv.env.AdminPass {
		return "admin", true
	}
	return "", false
}

func (srv *Server) reloadEnv() {
	InitFromEnvFile("/etc/qosnat2/env")
	srv.env.AdminUser = EnvOr("ADMIN_USER", srv.env.AdminUser)
	srv.env.AdminPass = EnvOr("ADMIN_PASS", srv.env.AdminPass)
	srv.env.AdminPort = EnvOr("ADMIN_PORT", srv.env.AdminPort)
	srv.env.DevLAN = EnvOr("DEV_LAN", "")
	srv.env.DevWAN = EnvOr("DEV_WAN", "")
	lan := srv.env.DevLAN
	if lan == "" {
		lan = "lo"
	}
	st := srv.store.Get()
	srv.hosts = shaper.NewHostShaper(lan, st.Shaper.Leaf)
}
