package acme

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-acme/lego/v4/registration"
)

func accountRegPath(staging bool) string {
	if staging {
		return filepath.Join(filepath.Dir(AccountRegPath), "account-staging.json")
	}
	return AccountRegPath
}

// registrationMatches 已缓存账户是否与当前 LE 环境（staging/production）一致。
func registrationMatches(reg *registration.Resource, staging bool) bool {
	if reg == nil || strings.TrimSpace(reg.URI) == "" {
		return false
	}
	u := strings.ToLower(reg.URI)
	if staging {
		return strings.Contains(u, "staging")
	}
	return strings.Contains(u, "acme-v02.api.letsencrypt.org") && !strings.Contains(u, "staging")
}

func loadRegistration(staging bool) (*registration.Resource, error) {
	path := accountRegPath(staging)
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var reg registration.Resource
	if err := json.Unmarshal(b, &reg); err != nil {
		return nil, err
	}
	if !registrationMatches(&reg, staging) {
		return nil, fmt.Errorf("registration environment mismatch")
	}
	return &reg, nil
}

func saveRegistration(reg *registration.Resource, staging bool) error {
	b, err := json.Marshal(reg)
	if err != nil {
		return err
	}
	return os.WriteFile(accountRegPath(staging), b, 0600)
}
