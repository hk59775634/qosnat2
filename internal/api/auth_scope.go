package api

import (
	"net/http"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func isWriteMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func (srv *Server) apiKeyWriteScopeError(r *http.Request) (code, message string) {
	if strings.TrimSpace(r.Header.Get("X-API-Key")) == "" {
		return "", ""
	}
	switch srv.apiKeyRoleFromRequest(r) {
	case store.APIKeyRoleReadOnly:
		return "AUTH_READ_ONLY", "read-only API key cannot modify configuration"
	case store.APIKeyRoleFirewall:
		if apiKeyFirewallWriteAllowed(r.URL.Path) {
			return "", ""
		}
		return "AUTH_SCOPE_FIREWALL", "firewall-scoped API key may only modify /api/v1/firewall/*"
	default:
		return "", ""
	}
}

func apiKeyFirewallWriteAllowed(path string) bool {
	return strings.HasPrefix(path, "/api/v1/firewall/")
}

func (srv *Server) apiKeyRoleFromRequest(r *http.Request) string {
	key := strings.TrimSpace(r.Header.Get("X-API-Key"))
	if key == "" {
		return store.APIKeyRoleAdmin
	}
	st := srv.store.Get()
	for _, k := range st.APIKeys {
		if k.KeyHash == "" {
			continue
		}
		if !store.VerifyAPIKey(key, k.KeyHash) {
			continue
		}
		return store.NormalizeAPIKeyRole(k.Role)
	}
	return store.APIKeyRoleAdmin
}
