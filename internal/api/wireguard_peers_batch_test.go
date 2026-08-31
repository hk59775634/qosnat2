package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestWireGuardPeersBatchDelete(t *testing.T) {
	srv := testServer(t)
	_ = srv.store.Update(func(s *store.State) {
		s.VPN.WireGuards = []store.WireGuardInstance{{
			ID:   "default",
			Name: "default",
			Mode: store.WGModeServer,
			WireGuardState: store.WireGuardState{
				Peers: []store.WGPeer{
					{Name: "p1", PublicKey: "a"},
					{Name: "p2", PublicKey: "b"},
					{Name: "p3", PublicKey: "c"},
				},
			},
		}}
		store.NormalizeWireGuardInstance(&s.VPN.WireGuards[0])
	})

	body, _ := json.Marshal(map[string]any{"names": []string{"p1", "p3", "missing"}})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/instances/default/peers/batch-delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleWireGuardInstancesSubtree(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200 got %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp["ok"] != true {
		t.Fatalf("resp: %+v", resp)
	}
	deleted, _ := resp["deleted"].([]any)
	if len(deleted) != 2 {
		t.Fatalf("deleted: %+v", deleted)
	}
	notFound, _ := resp["not_found"].([]any)
	if len(notFound) != 1 || notFound[0] != "missing" {
		t.Fatalf("not_found: %+v", notFound)
	}
	st := srv.store.Get()
	if len(st.VPN.WireGuards[0].Peers) != 1 || st.VPN.WireGuards[0].Peers[0].Name != "p2" {
		t.Fatalf("peers: %+v", st.VPN.WireGuards[0].Peers)
	}
}

func TestWireGuardPeersBatchDeleteEmpty(t *testing.T) {
	srv := testServer(t)
	body := `{"names":[]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vpn/wireguard/instances/default/peers/batch-delete", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.handleWireGuardInstancesSubtree(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("want 400 got %d", w.Code)
	}
}
