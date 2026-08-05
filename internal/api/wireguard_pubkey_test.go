package api

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

func TestDeriveWireGuardPublicKey(t *testing.T) {
	kp, err := wg.GenKeyPair()
	if err != nil {
		t.Fatal(err)
	}
	st := &store.WireGuardState{PrivateKey: kp.Private, PublicKey: "stale"}
	if err := deriveWireGuardPublicKey(st); err != nil {
		t.Fatal(err)
	}
	if st.PublicKey != kp.Public {
		t.Fatalf("public=%q want %q", st.PublicKey, kp.Public)
	}
}

func TestDeriveWireGuardPublicKey_emptyKeeps(t *testing.T) {
	st := &store.WireGuardState{PrivateKey: "", PublicKey: "keep"}
	if err := deriveWireGuardPublicKey(st); err != nil {
		t.Fatal(err)
	}
	if st.PublicKey != "keep" {
		t.Fatalf("empty private should not change public, got %q", st.PublicKey)
	}
}

func TestDeriveWireGuardPublicKey_invalid(t *testing.T) {
	st := &store.WireGuardState{PrivateKey: "not-a-key"}
	err := deriveWireGuardPublicKey(st)
	if err == nil || !strings.Contains(err.Error(), "private_key") {
		t.Fatalf("expected private_key error, got %v", err)
	}
}
