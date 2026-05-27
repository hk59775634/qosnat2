package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestHandleNptv6Validation(t *testing.T) {
	srv := testServer(t)
	body := `{"nptv6_enabled":true,"nptv6_rules":[{"internal_prefix":"fd00::/48","external_prefix":"2001:db8::/64"}]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nat/nptv6", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleNptv6(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d %s", w.Code, w.Body.String())
	}
}

func TestHandleNat64Disabled(t *testing.T) {
	srv := testServer(t)
	body := `{"nat64_enabled":false,"dns64":{"mode":"local_unbound"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nat/nat64", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleNat64(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected %d %s", w.Code, w.Body.String())
	}
}

func testServer(t *testing.T) *Server {
	t.Helper()
	st := store.New(t.TempDir() + "/state.json")
	st.State.SetupComplete = true
	st.State.Nat = store.DefaultNat()
	srv := &Server{store: st, env: Env{DevLAN: "lo", DevWAN: "lo", AdminPort: "8080"}}
	return srv
}

func TestNatStateRoundTrip(t *testing.T) {
	var s store.State
	raw := `{"nat":{"ipv4":{"policy_routes":["10.0.0.0/8"]},"nat64_enabled":true,"dns64":{"mode":"upstream"}}}`
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		t.Fatal(err)
	}
	if !s.Nat.Nat64Enabled || s.Nat.DNS64.Mode != store.DNS64ModeUpstream {
		t.Fatalf("nat: %+v", s.Nat)
	}
}
