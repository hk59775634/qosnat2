package api

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

var safeFilenameRe = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return strings.TrimSpace(r.RemoteAddr)
	}
	return host
}

func isLoopback(r *http.Request) bool {
	ip := net.ParseIP(clientIP(r))
	return ip != nil && ip.IsLoopback()
}

func (srv *Server) tlsActive() bool {
	st := srv.store.Get()
	if !st.System.TLSEnabled {
		return false
	}
	if srv.env.TLSCert == "" || srv.env.TLSKey == "" {
		return false
	}
	if _, err := os.Stat(srv.env.TLSCert); err != nil {
		return false
	}
	_, err := os.Stat(srv.env.TLSKey)
	return err == nil
}

func (srv *Server) listenAddr() string {
	port := srv.env.AdminPort
	if srv.tlsActive() {
		return ":" + port
	}
	return "127.0.0.1:" + port
}

func (srv *Server) setSessionCookie(w http.ResponseWriter, r *http.Request, tok string) {
	c := &http.Cookie{
		Name:     sessionCookie,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   int(sessionTTL.Seconds()),
	}
	if srv.tlsActive() {
		c.Secure = true
		c.SameSite = http.SameSiteStrictMode
	} else {
		c.SameSite = http.SameSiteStrictMode
	}
	http.SetCookie(w, c)
}

func safeAttachmentFilename(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" || !safeFilenameRe.MatchString(name) {
		return "", fmt.Errorf("invalid filename")
	}
	return name, nil
}

func setupTokenFromRequest(r *http.Request, bodyToken string) string {
	if h := strings.TrimSpace(r.Header.Get("X-Setup-Token")); h != "" {
		return h
	}
	return strings.TrimSpace(bodyToken)
}

func (srv *Server) verifySetupToken(tok string) bool {
	tok = strings.TrimSpace(tok)
	if tok == "" {
		return false
	}
	st := srv.store.Get()
	if st.SetupComplete || st.SetupToken == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(tok), []byte(st.SetupToken)) == 1
}

func (srv *Server) requireSetupAccess(w http.ResponseWriter, r *http.Request, bodyToken string) bool {
	if srv.setupComplete() {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "setup already complete"})
		return false
	}
	if !isLoopback(r) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "setup only allowed from localhost"})
		return false
	}
	if !srv.verifySetupToken(setupTokenFromRequest(r, bodyToken)) {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "invalid or missing setup token"})
		return false
	}
	return true
}

// EnsureSetupToken 未完成引导时生成一次性 token 并持久化
func (srv *Server) EnsureSetupToken() error {
	if srv.setupComplete() {
		return nil
	}
	st := srv.store.Get()
	if st.SetupToken != "" {
		return nil
	}
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return err
	}
	tok := hex.EncodeToString(b[:])
	return srv.store.Update(func(s *store.State) {
		s.SetupToken = tok
	})
}

func newLoginLimiter() *loginLimiter {
	return &loginLimiter{fail: map[string]loginFail{}}
}

type loginFail struct {
	count int
	until time.Time
}

type loginLimiter struct {
	mu   sync.Mutex
	fail map[string]loginFail
}

func (l *loginLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.fail[ip]
	if !ok {
		return true
	}
	if time.Now().Before(f.until) {
		return false
	}
	delete(l.fail, ip)
	return true
}

func (l *loginLimiter) recordFail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	f := l.fail[ip]
	f.count++
	if f.count >= 10 {
		f.until = time.Now().Add(15 * time.Minute)
		f.count = 0
	}
	l.fail[ip] = f
}

func (l *loginLimiter) clear(ip string) {
	l.mu.Lock()
	delete(l.fail, ip)
	l.mu.Unlock()
}

func validateCaptureDevice(dev string) error {
	return netif.ValidateIfaceName(dev)
}

func sanitizeTcpdumpFilter(filter string) error {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		return nil
	}
	if strings.HasPrefix(filter, "-") {
		return fmt.Errorf("filter must not start with '-'")
	}
	return nil
}
