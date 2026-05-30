package api

import (
	"testing"
	"time"
)

func TestVersionSwitchGrantsConsumeOnce(t *testing.T) {
	g := newVersionSwitchGrants()
	g.grant("sess-1")
	if !g.consume("sess-1") {
		t.Fatal("expected grant to be valid")
	}
	if g.consume("sess-1") {
		t.Fatal("grant should be single-use")
	}
}

func TestVersionSwitchGrantsExpire(t *testing.T) {
	g := newVersionSwitchGrants()
	g.mu.Lock()
	g.until["sess-2"] = time.Now().Add(-time.Second)
	g.mu.Unlock()
	if g.consume("sess-2") {
		t.Fatal("expired grant should not be accepted")
	}
}
