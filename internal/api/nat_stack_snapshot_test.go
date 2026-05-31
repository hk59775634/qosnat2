package api

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestLastNatStackSnapshotEmptyUntilSuccess(t *testing.T) {
	srv := &Server{}
	if snap := srv.lastNatStackSnapshot(); snap.Nat.Nat64Enabled {
		t.Fatal("expected empty snapshot before first success")
	}
	st := store.State{
		Nat:  store.NatState{Nat64Enabled: true},
		DHCP: store.DefaultDHCP(),
	}
	srv.recordNatStackSuccess(st)
	snap := srv.lastNatStackSnapshot()
	if !snap.Nat.Nat64Enabled {
		t.Fatal("expected recorded nat64 enabled")
	}
}

func TestTerminalGrantsConsumeOnce(t *testing.T) {
	g := newVersionSwitchGrants()
	g.grant("sess-t")
	if !g.consume("sess-t") {
		t.Fatal("first consume should succeed")
	}
	if g.consume("sess-t") {
		t.Fatal("second consume should fail")
	}
}
