package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) handleAPIKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		keys := st.APIKeys
		if keys == nil {
			keys = []store.APIKey{}
		}
		type item struct {
			ID        string `json:"id"`
			Name      string `json:"name"`
			Role      string `json:"role"`
			CreatedAt string `json:"created_at"`
			Prefix    string `json:"key_prefix"`
		}
		out := make([]item, 0, len(keys))
		for _, k := range keys {
			pfx := k.KeyPrefix
			if pfx == "" && k.KeyHash != "" {
				pfx = k.KeyHash[:8] + "…"
			}
			out = append(out, item{
				ID: k.ID, Name: k.Name, Role: store.NormalizeAPIKeyRole(k.Role),
				CreatedAt: k.CreatedAt, Prefix: pfx,
			})
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var body struct {
			Name string `json:"name"`
			Role string `json:"role"`
		}
		if err := readJSON(r, &body); err != nil || body.Name == "" {
			writeBadRequest(w, "name required")
			return
		}
		raw := make([]byte, 24)
		if _, err := rand.Read(raw); err != nil {
			writeInternalError(w, "session error")
			return
		}
		key := "qk_" + hex.EncodeToString(raw)
		keyHash, err := store.HashAPIKey(key)
		if err != nil {
			writeInternalError(w, "hash error")
			return
		}
		ak := store.APIKey{
			ID:        "key-" + hex.EncodeToString(raw[:8]),
			Name:      body.Name,
			Role:      store.NormalizeAPIKeyRole(body.Role),
			KeyHash:   keyHash,
			KeyPrefix: store.APIKeyPrefix(key),
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		}
		_ = srv.store.Update(func(st *store.State) {
			st.APIKeys = append(st.APIKeys, ak)
		})
		if !srv.persistState(w) {
			return
		}
		srv.auditLog(r, "apikey.create", ak.Name)
		writeJSON(w, http.StatusCreated, map[string]any{
			"id": ak.ID, "name": ak.Name, "role": ak.Role, "key": key, "created_at": ak.CreatedAt,
		})
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeBadRequest(w, "id required")
			return
		}
		found := false
		_ = srv.store.Update(func(st *store.State) {
			var out []store.APIKey
			for _, k := range st.APIKeys {
				if k.ID == id {
					found = true
					continue
				}
				out = append(out, k)
			}
			st.APIKeys = out
		})
		if !found {
			writeNotFound(w, "not found")
			return
		}
		if !srv.persistState(w) {
			return
		}
		srv.auditLog(r, "apikey.delete", id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		writeMethodNotAllowed(w)
	}
}
