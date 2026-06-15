package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

func writeSessionFile(path string, data []byte) error {
	return store.WriteFileAtomic(path, data, 0600)
}

const (
	sessionCookie  = "qosnat_sess"
	sessionHeader  = "X-Qosnat-Session"
	sessionTTL     = 30 * 24 * time.Hour
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return json.Unmarshal(b, &s.sessions)
}

func (s *sessionStore) valid(tok string) bool {
	if tok == "" {
		return false
	}
	s.mu.Lock()
	ok := s.validLocked(tok)
	s.mu.Unlock()
	if ok {
		return true
	}
	if err := s.load(); err != nil {
		log.Printf("session reload: %v", err)
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.validLocked(tok)
}

func (s *sessionStore) validLocked(tok string) bool {
	s.pruneLocked()
	exp, ok := s.sessions[tok]
	return ok && time.Now().Before(exp)
}

func (s *sessionStore) saveLocked() error {
	b, err := json.Marshal(s.sessions)
	if err != nil {
		return err
	}
	return writeSessionFile(s.file, b)
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

func (s *sessionStore) delete(tok string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, tok)
	_ = s.saveLocked()
}

func sessionTokenFromRequest(r *http.Request) string {
	if c, err := r.Cookie(sessionCookie); err == nil {
		if v := strings.TrimSpace(c.Value); v != "" {
			return v
		}
	}
	if h := strings.TrimSpace(r.Header.Get(sessionHeader)); h != "" {
		return h
	}
	if h := strings.TrimSpace(r.Header.Get("Authorization")); len(h) > 7 && strings.EqualFold(h[:7], "bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

func (srv *Server) requestAuthorized(r *http.Request) bool {
	if srv.checkAPIKey(r) {
		return true
	}
	return srv.sessions.valid(sessionTokenFromRequest(r))
}

func (srv *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !srv.requestAuthorized(r) {
			writeUnauthorized(w, "unauthorized")
			return
		}
		if isWriteMethod(r.Method) {
			if code, msg := srv.apiKeyWriteScopeError(r); code != "" {
				writeAPIError(w, http.StatusForbidden, code, msg)
				return
			}
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
		if k.KeyHash == "" {
			continue
		}
		if !store.VerifyAPIKey(key, k.KeyHash) {
			continue
		}
		if store.IsLegacyAPIKeyHash(k.KeyHash) {
			if nh, err := store.HashAPIKey(key); err == nil {
				id := k.ID
				_ = srv.store.Update(func(s *store.State) {
					for j := range s.APIKeys {
						if s.APIKeys[j].ID == id {
							s.APIKeys[j].KeyHash = nh
							break
						}
					}
				})
				if err := srv.store.Save(); err != nil {
					log.Printf("upgrade api key hash: save: %v", err)
				}
			}
		}
		return true
	}
	return false
}
