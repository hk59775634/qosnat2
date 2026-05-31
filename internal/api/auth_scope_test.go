package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestReadOnlyAPIKeyBlocksWrite(t *testing.T) {
	plain := "qk_readonly_test_key_abc"
	hash, err := store.HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(t.TempDir() + "/state.json")
	st.State.SetupComplete = true
	st.State.APIKeys = []store.APIKey{{
		ID: "key-ro", Name: "ro", Role: store.APIKeyRoleReadOnly, KeyHash: hash,
	}}
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo", AdminPort: "8080"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/firewall/rules", bytes.NewBufferString(`{}`))
	req.Header.Set("X-API-Key", plain)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestReadOnlyAPIKeyAllowsGet(t *testing.T) {
	plain := "qk_readonly_get_key"
	hash, err := store.HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(t.TempDir() + "/state.json")
	st.State.APIKeys = []store.APIKey{{
		ID: "key-ro2", Name: "ro", Role: store.APIKeyRoleReadOnly, KeyHash: hash,
	}}
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/ops", nil)
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	srv.requireAuth(srv.handleMetricsOps)(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d %s", w.Code, w.Body.String())
	}
}

func TestFirewallAPIKeyAllowsFirewallWrite(t *testing.T) {
	plain := "qk_firewall_scope_key"
	hash, err := store.HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(t.TempDir() + "/state.json")
	st.State.APIKeys = []store.APIKey{{
		ID: "key-fw", Name: "fw", Role: store.APIKeyRoleFirewall, KeyHash: hash,
	}}
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/firewall/rules", bytes.NewBufferString(`{}`))
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d %s", w.Code, w.Body.String())
	}
}

func TestFirewallAPIKeyBlocksNatWrite(t *testing.T) {
	plain := "qk_firewall_scope_key2"
	hash, err := store.HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(t.TempDir() + "/state.json")
	st.State.APIKeys = []store.APIKey{{
		ID: "key-fw2", Name: "fw", Role: store.APIKeyRoleFirewall, KeyHash: hash,
	}}
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/nat/static-mappings", bytes.NewBufferString(`{}`))
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	srv.requireAuth(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	})(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}
