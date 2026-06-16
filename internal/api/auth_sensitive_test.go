package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestReadOnlyAPIKeyBlockedFromStateExport(t *testing.T) {
	plain := "qk_readonly_export_key"
	hash, err := store.HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(t.TempDir() + "/state.json")
	st.State.SetupComplete = true
	st.State.AdminUser = "admin"
	hashPW, _ := hashPassword("admin-secret")
	st.State.AdminPassHash = string(hashPW)
	st.State.APIKeys = []store.APIKey{{
		ID: "key-ro", Name: "ro", Role: store.APIKeyRoleReadOnly, KeyHash: hash,
	}}
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/state/export", bytes.NewBufferString(`{"current_password":"admin-secret"}`))
	req.Header.Set("X-API-Key", plain)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.requireAuth(srv.handleSystemStateExport)(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}

func TestStateExportRequiresPassword(t *testing.T) {
	st := store.New(t.TempDir() + "/state.json")
	st.State.AdminUser = "admin"
	hashPW, _ := hashPassword("admin-secret")
	st.State.AdminPassHash = string(hashPW)
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/system/state/export", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleSystemStateExport(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 without password, got %d", w.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/v1/system/state/export", bytes.NewBufferString(`{"current_password":"admin-secret"}`))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.handleSystemStateExport(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 with password, got %d %s", w.Code, w.Body.String())
	}
}

func TestStateImportRawRejectsPasswordQuery(t *testing.T) {
	st := store.New(t.TempDir() + "/state.json")
	st.State.AdminUser = "admin"
	hashPW, _ := hashPassword("admin-secret")
	st.State.AdminPassHash = string(hashPW)
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/system/state/import/raw?current_password=admin-secret", bytes.NewBufferString(`{}`))
	w := httptest.NewRecorder()
	srv.handleSystemStateImportRaw(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 for query password, got %d %s", w.Code, w.Body.String())
	}
}

func TestReadOnlyAPIKeyBlockedFromTerminalScope(t *testing.T) {
	plain := "qk_readonly_term_key"
	hash, err := store.HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	st := store.New(t.TempDir() + "/state.json")
	st.State.APIKeys = []store.APIKey{{
		ID: "key-ro", Name: "ro", Role: store.APIKeyRoleReadOnly, KeyHash: hash,
	}}
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo"}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/diagnostics/terminal", nil)
	req.Header.Set("X-API-Key", plain)
	w := httptest.NewRecorder()
	srv.handleTerminalWS(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403 got %d %s", w.Code, w.Body.String())
	}
}
