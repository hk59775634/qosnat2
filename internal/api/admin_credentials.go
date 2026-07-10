package api

import (
	"fmt"
	"os"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

// ChangeAdminUser 更新管理员用户名并重启 qosnatd（需 root）。
func ChangeAdminUser(newUser string) (string, error) {
	newUser, err := validateAdminUsername(newUser)
	if err != nil {
		return "", err
	}
	env := LoadEnv()
	st := store.New(env.StateFile)
	if err := st.Load(); err != nil {
		return "", fmt.Errorf("load state: %w", err)
	}
	cur := effectiveAdminUser(st.Get(), env)
	if cur == newUser {
		return newUser, nil
	}
	env.AdminUser = newUser
	if err := WriteRuntimeEnv(env); err != nil {
		return "", err
	}
	updateInitialAdminFileKey("ADMIN_USER", newUser)
	if err := st.Update(func(s *store.State) {
		s.AdminUser = newUser
	}); err != nil {
		return "", err
	}
	if err := st.Save(); err != nil {
		return "", fmt.Errorf("save state: %w", err)
	}
	if err := restartQoSnatd(); err != nil {
		return "", err
	}
	return newUser, nil
}

// ChangeAdminPassword 更新管理员密码并重启 qosnatd（需 root）。
func ChangeAdminPassword(newPass string) error {
	if len(newPass) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	hash, err := hashPassword(newPass)
	if err != nil {
		return err
	}
	env := LoadEnv()
	st := store.New(env.StateFile)
	if err := st.Load(); err != nil {
		return fmt.Errorf("load state: %w", err)
	}
	user := effectiveAdminUser(st.Get(), env)
	if err := st.Update(func(s *store.State) {
		s.AdminUser = user
		s.AdminPassHash = string(hash)
	}); err != nil {
		return err
	}
	if err := st.Save(); err != nil {
		return fmt.Errorf("save state: %w", err)
	}
	env.AdminUser = user
	env.AdminPass = ""
	if err := WriteRuntimeEnv(env); err != nil {
		return err
	}
	_ = ClearAdminPassFromEnv()
	updateInitialAdminFileKey("ADMIN_PASS", newPass)
	return restartQoSnatd()
}

func effectiveAdminUser(st store.State, env Env) string {
	if u := strings.TrimSpace(st.AdminUser); u != "" {
		return u
	}
	if u := strings.TrimSpace(env.AdminUser); u != "" {
		return u
	}
	return defaultAdminUser
}

func validateAdminUsername(user string) (string, error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return "", fmt.Errorf("username required")
	}
	if strings.ContainsAny(user, " \t\n\r\x00") {
		return "", fmt.Errorf("username must not contain whitespace")
	}
	return user, nil
}

func updateInitialAdminFileKey(key, value string) {
	b, err := os.ReadFile(initialAdminFile)
	if err != nil {
		return
	}
	lines := strings.Split(string(b), "\n")
	found := false
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if trim == "" || strings.HasPrefix(trim, "#") {
			continue
		}
		k, _, ok := strings.Cut(trim, "=")
		if ok && strings.TrimSpace(k) == key {
			lines[i] = key + "=" + value
			found = true
			break
		}
	}
	if !found {
		lines = append(lines, key+"="+value)
	}
	_ = os.WriteFile(initialAdminFile, []byte(strings.Join(lines, "\n")), 0600)
}
