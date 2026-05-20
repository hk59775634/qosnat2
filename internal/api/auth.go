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
)

const (
	sessionCookie = "qosnat_sess"
	sessionTTL    = 30 * 24 * time.Hour
)

type sessionStore struct {
	mu       sync.RWMutex
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

func (s *sessionStore) save() error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	b, err := json.Marshal(s.sessions)
	if err != nil {
		return err
	}
	return os.WriteFile(s.file, b, 0600)
}

func (s *sessionStore) create() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[tok] = time.Now().Add(sessionTTL)
	s.mu.Unlock()
	return tok, s.save()
}

func (s *sessionStore) valid(tok string) bool {
	s.mu.RLock()
	exp, ok := s.sessions[tok]
	s.mu.RUnlock()
	return ok && time.Now().Before(exp)
}

func (s *sessionStore) delete(tok string) {
	s.mu.Lock()
	delete(s.sessions, tok)
	s.mu.Unlock()
	_ = s.save()
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
	st := srv.store.Get()
	for _, k := range st.APIKeys {
		if subtle.ConstantTimeCompare([]byte(k.Key), []byte(key)) == 1 {
			return true
		}
	}
	return false
}
