package api

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestMergeOCServRadiusSecretBeforeNormalize(t *testing.T) {
	prev := store.DefaultOCServ()
	prev.AuthMethod = store.OCServAuthRadius
	prev.Radius.Server = "10.0.0.1"
	prev.Radius.Secret = "saved-secret"

	body := prev
	body.Radius.Secret = ""

	mergeOCServRadiusSecret(&body, prev)
	if err := store.NormalizeOCServ(&body); err != nil {
		t.Fatalf("normalize after merge: %v", err)
	}
	if body.Radius.Secret != "saved-secret" {
		t.Fatalf("secret = %q, want saved-secret", body.Radius.Secret)
	}
}

func TestMergeAllOCServVhostUserPasswords(t *testing.T) {
	prev := store.DefaultOCServ()
	prev.Vhosts = []store.OCServVhost{{
		Domain:  "vpn.example.com",
		Enabled: true,
		Users:   []store.OCServUser{{Username: "alice", Password: "saved-pw"}},
	}}
	body := prev
	body.Vhosts = []store.OCServVhost{{
		Domain:  "vpn.example.com",
		Enabled: true,
		Users:   []store.OCServUser{{Username: "alice"}},
	}}

	mergeAllOCServVhostUserPasswords(&body, prev)
	if body.Vhosts[0].Users[0].Password != "saved-pw" {
		t.Fatalf("password = %q, want saved-pw", body.Vhosts[0].Users[0].Password)
	}
}
