package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	sessionCookie = "qosnat_sess"
	sessionTTL    = 30 * 24 * time.Hour
)

type sessionStore struct {
	mu       sync.Mutex
	sessions map[string]time.Time
	file     string
}

func newSessionStore(path string) *sessionStore {
	return &sessionStore{sessions: map[string]time.Time{}, file: path}
}

func (s *sessionStore) load() error {
	b, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return json.Unmarshal(b, &s.sessions)
}

func (s *sessionStore) saveLocked() error {
	b, err := json.Marshal(s.sessions)
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0600)
}

func (s *sessionStore) pruneLocked() {
	now := time.Now()
	for tok, exp := range s.sessions {
		if !now.Before(exp) {
			delete(s.sessions, tok)
		}
	}
}

func (s *sessionStore) create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	s.sessions[tok] = time.Now().Add(sessionTTL)
	return tok, s.saveLocked()
}

func (s *sessionStore) valid(tok string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pruneLocked()
	exp, ok := s.sessions[tok]
	return ok && time.Now().Before(exp)
}

func (s *sessionStore) delete(tok string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tok)
	_ = s.saveLocked()
}

func (srv *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if srv.checkAPIKey(r) {
			next(w, r)
			return
		}
		c, err := r.Cookie(sessionCookie)
		if err != nil || !srv.sessions.valid(c.Value) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		next(w, r)
	}
}

func (srv *Server) checkAPIKey(r *http.Request) bool {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return false
	}
	hash := store.HashAPIKey(key)
	st := srv.store.Get()
	for _, k := range st.APIKeys {
		if k.KeyHash == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(k.KeyHash), []byte(hash)) == 1 {
			return true
		}
	}
	return false
}
