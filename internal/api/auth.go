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
	roleAdmin     = "admin"
	roleReadonly  = "readonly"
)

type sessionEntry struct {
	Expires time.Time `json:"exp"`
	Role    string    `json:"role,omitempty"`
}

type sessionStore struct {
	mu       sync.RWMutex
	sessions map[string]sessionEntry
	file     string
}

func newSessionStore(path string) *sessionStore {
	return &sessionStore{sessions: map[string]sessionEntry{}, file: path}
}

func (s *sessionStore) load() error {
	b, err := os.ReadFile(s.file)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var modern map[string]sessionEntry
	if err := json.Unmarshal(b, &modern); err == nil && len(modern) > 0 {
		s.mu.Lock()
		s.sessions = modern
		s.mu.Unlock()
		return nil
	}
	var legacy map[string]time.Time
	if err := json.Unmarshal(b, &legacy); err != nil {
		return err
	}
	s.mu.Lock()
	for tok, exp := range legacy {
		s.sessions[tok] = sessionEntry{Expires: exp, Role: roleAdmin}
	}
	s.mu.Unlock()
	return nil
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

func (s *sessionStore) create(role string) (string, error) {
	if role == "" {
		role = roleAdmin
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(b)
	s.mu.Lock()
	s.sessions[tok] = sessionEntry{Expires: time.Now().Add(sessionTTL), Role: role}
	s.mu.Unlock()
	return tok, s.save()
}

func (s *sessionStore) valid(tok string) bool {
	_, ok := s.entry(tok)
	return ok
}

func (s *sessionStore) role(tok string) string {
	e, ok := s.entry(tok)
	if !ok {
		return ""
	}
	if e.Role == "" {
		return roleAdmin
	}
	return e.Role
}

func (s *sessionStore) entry(tok string) (sessionEntry, bool) {
	s.mu.RLock()
	e, ok := s.sessions[tok]
	s.mu.RUnlock()
	if !ok || time.Now().After(e.Expires) {
		return sessionEntry{}, false
	}
	return e, true
}

func (s *sessionStore) delete(tok string) {
	s.mu.Lock()
	delete(s.sessions, tok)
	s.mu.Unlock()
	_ = s.save()
}

func (srv *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		role := srv.authRole(r)
		if role == "" {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if role == roleReadonly && r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "read-only account cannot modify configuration"})
			return
		}
		next(w, r)
	}
}

func (srv *Server) authRole(r *http.Request) string {
	if role, ok := srv.apiKeyRole(r); ok {
		return role
	}
	c, err := r.Cookie(sessionCookie)
	if err != nil || !srv.sessions.valid(c.Value) {
		return ""
	}
	return srv.sessions.role(c.Value)
}

func (srv *Server) apiKeyRole(r *http.Request) (string, bool) {
	key := r.Header.Get("X-API-Key")
	if key == "" {
		return "", false
	}
	st := srv.store.Get()
	for _, k := range st.APIKeys {
		if subtle.ConstantTimeCompare([]byte(k.Key), []byte(key)) == 1 {
			role := k.Role
			if role == "" {
				role = roleAdmin
			}
			return role, true
		}
	}
	return "", false
}

func (srv *Server) checkAPIKey(r *http.Request) bool {
	_, ok := srv.apiKeyRole(r)
	return ok
}
