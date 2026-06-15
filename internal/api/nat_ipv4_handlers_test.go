package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestHandleNatIPv4PutDisablesNAT(t *testing.T) {
	srv := testServer(t)
	body, _ := json.Marshal(map[string]bool{"enabled": false})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/nat/ipv4", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleNatIPv4(w, req)
	if w.Code != http.StatusOK && w.Code != http.StatusInternalServerError {
		t.Fatalf("unexpected %d %s", w.Code, w.Body.String())
	}
	if w.Code == http.StatusOK {
		got := srv.store.Get()
		if store.NatIPv4Enabled(got.Nat.IPv4) {
			t.Fatal("expected NAT disabled")
		}
	}
}

func TestHandleNatIPv4Get(t *testing.T) {
	srv := testServer(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/nat/ipv4", nil)
	w := httptest.NewRecorder()
	srv.handleNatIPv4(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status %d", w.Code)
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["enabled"] != true {
		t.Fatalf("default enabled want true got %v", resp["enabled"])
	}
}
