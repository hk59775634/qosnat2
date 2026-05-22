package api

import (
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
	return srv.store.Get().SetupComplete
}

func (srv *Server) verifyAdmin(user, pass string) bool {
	st := srv.store.Get()
	if st.AdminPassHash == "" || st.AdminUser == "" {
		return false
	}
	return user == st.AdminUser && checkPassword(st.AdminPassHash, pass)
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
