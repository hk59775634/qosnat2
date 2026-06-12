package api

import (
	"strings"

	"golang.org/x/crypto/bcrypt"
)

const defaultAdminUser = "admin"

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

func (srv *Server) initialAdminCredentials() (user, pass string) {
	user = strings.TrimSpace(srv.env.AdminUser)
	if user == "" {
		user = defaultAdminUser
	}
	pass = srv.env.AdminPass
	return user, pass
}

func (srv *Server) setupComplete() bool {
	return srv.store.Get().SetupComplete
}

// verifyAdmin 已设置 bcrypt 时用 state；否则仅接受 env 中的安装随机口令（无默认弱口令）。
func (srv *Server) verifyAdmin(user, pass string) bool {
	user = strings.TrimSpace(user)
	st := srv.store.Get()
	if st.AdminPassHash != "" && st.AdminUser != "" {
		return user == st.AdminUser && checkPassword(st.AdminPassHash, pass)
	}
	iu, ip := srv.initialAdminCredentials()
	if ip == "" {
		return false
	}
	return user == iu && pass == ip
}

func (srv *Server) reloadEnv() {
	InitFromEnvFile("/etc/qosnat2/env")
	lan, wan := syncDevRolesFromFile()
	srv.env.AdminUser = EnvOr("ADMIN_USER", srv.env.AdminUser)
	srv.env.AdminPass = EnvOr("ADMIN_PASS", srv.env.AdminPass)
	srv.env.AdminPort = EnvOr("ADMIN_PORT", srv.env.AdminPort)
	srv.env.DevLAN = lan
	srv.env.DevWAN = wan
}
