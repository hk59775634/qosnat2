package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

// 确保 system/version 路由已注册（General.vue 依赖）。
func TestRoutesSystemVersionRegistered(t *testing.T) {
	st := store.New(t.TempDir() + "/state.json")
	srv := New(Env{AdminPort: "8080"}, st, nil)
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/version", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode == http.StatusNotFound {
		t.Fatal("/api/v1/system/version returned 404 (route not registered)")
	}

	reqV, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/system/version/switch/verify", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	reqV.Header.Set("Content-Type", "application/json")
	resV, err := http.DefaultClient.Do(reqV)
	if err != nil {
		t.Fatal(err)
	}
	resV.Body.Close()
	if resV.StatusCode == http.StatusNotFound {
		t.Fatal("/api/v1/system/version/switch/verify returned 404 (route not registered)")
	}

	req2, err := http.NewRequest(http.MethodPost, ts.URL+"/api/v1/system/version/switch", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	req2.Header.Set("Content-Type", "application/json")
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatal(err)
	}
	res2.Body.Close()
	if res2.StatusCode == http.StatusNotFound {
		t.Fatal("/api/v1/system/version/switch returned 404 (route not registered)")
	}

	req3, err := http.NewRequest(http.MethodGet, ts.URL+"/api/v1/system/version/switch/status", nil)
	if err != nil {
		t.Fatal(err)
	}
	res3, err := http.DefaultClient.Do(req3)
	if err != nil {
		t.Fatal(err)
	}
	res3.Body.Close()
	if res3.StatusCode == http.StatusNotFound {
		t.Fatal("/api/v1/system/version/switch/status returned 404 (route not registered)")
	}
}
