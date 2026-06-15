package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestLoginSessionCookieAndHeader(t *testing.T) {
	dir := t.TempDir()
	st := store.New(filepath.Join(dir, "state.json"))
	st.State.SetupComplete = true
	st.State.AdminUser = "admin"
	hash, err := hashPassword("secret")
	if err != nil {
		t.Fatal(err)
	}
	st.State.AdminPassHash = string(hash)

	srv := New(Env{
		AdminUser:   "admin",
		AdminPort:   "8080",
		SessionFile: filepath.Join(dir, "sessions.json"),
	}, st, nil)
	_ = srv.sessions.load()

	loginReq := httptest.NewRequest(http.MethodPost, "/api/v1/login", bytes.NewBufferString(`{"user":"admin","pass":"secret"}`))
	loginReq.Header.Set("Content-Type", "application/json")
	loginW := httptest.NewRecorder()
	srv.handleLogin(loginW, loginReq)
	if loginW.Code != http.StatusOK {
		t.Fatalf("login status=%d body=%s", loginW.Code, loginW.Body.String())
	}
	cookies := loginW.Result().Cookies()
	var tok string
	for _, c := range cookies {
		if c.Name == sessionCookie {
			tok = c.Value
		}
	}
	if tok == "" {
		t.Fatal("missing session cookie")
	}

	sessReq := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	sessReq.AddCookie(&http.Cookie{Name: sessionCookie, Value: tok})
	sessW := httptest.NewRecorder()
	srv.requireAuth(srv.handleSession)(sessW, sessReq)
	if sessW.Code != http.StatusOK {
		t.Fatalf("session with cookie status=%d", sessW.Code)
	}

	sessReq2 := httptest.NewRequest(http.MethodGet, "/api/v1/session", nil)
	sessReq2.Header.Set(sessionHeader, tok)
	sessW2 := httptest.NewRecorder()
	srv.requireAuth(srv.handleSession)(sessW2, sessReq2)
	if sessW2.Code != http.StatusOK {
		t.Fatalf("session with header status=%d", sessW2.Code)
	}
}
