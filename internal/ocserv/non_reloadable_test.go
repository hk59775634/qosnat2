package ocserv

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestNonReloadableChangeReasons_tcpPort(t *testing.T) {
	prev := store.DefaultOCServ()
	next := store.DefaultOCServ()
	next.TCPPort = 444
	r := NonReloadableChangeReasons(prev, next, nil)
	if len(r) != 1 || r[0] != "listen_ports" {
		t.Fatalf("got %v want [listen_ports]", r)
	}
}

func TestNonReloadableChangeReasons_same(t *testing.T) {
	o := store.DefaultOCServ()
	if len(NonReloadableChangeReasons(o, o, nil)) != 0 {
		t.Fatal("expected no diff")
	}
}

func TestNonReloadableChangeReasons_radiusServer(t *testing.T) {
	prev := store.DefaultOCServ()
	prev.AuthMethod = store.OCServAuthRadius
	prev.Radius.Server = "10.0.0.1"
	prev.Radius.Secret = "s"
	next := prev
	next.Radius.Server = "10.0.0.2"
	r := NonReloadableChangeReasons(prev, next, nil)
	if len(r) != 1 || r[0] != "auth_global" {
		t.Fatalf("got %v want [auth_global]", r)
	}
}

func TestNonReloadableChangeReasons_authSwitch(t *testing.T) {
	prev := store.DefaultOCServ()
	next := store.DefaultOCServ()
	next.AuthMethod = store.OCServAuthRadius
	next.Radius.Server = "10.0.0.1"
	next.Radius.Secret = "secret"
	r := NonReloadableChangeReasons(prev, next, nil)
	hasAuth := false
	for _, x := range r {
		if x == "auth_global" {
			hasAuth = true
		}
	}
	if !hasAuth {
		t.Fatalf("expected auth_global in %v", r)
	}
}
