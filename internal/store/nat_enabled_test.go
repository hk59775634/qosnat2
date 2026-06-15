package store

import "testing"

func TestNatIPv4EnabledDefault(t *testing.T) {
	if !NatIPv4Enabled(NatIPv4State{}) {
		t.Fatal("nil enabled should default to true")
	}
	on := true
	if !NatIPv4Enabled(NatIPv4State{Enabled: &on}) {
		t.Fatal("explicit true")
	}
	off := false
	if NatIPv4Enabled(NatIPv4State{Enabled: &off}) {
		t.Fatal("explicit false")
	}
}

func TestCloneNatIPv4PreservesEnabled(t *testing.T) {
	off := false
	src := NatIPv4State{Enabled: &off, PolicyRoutes: []string{"10.0.0.0/8"}}
	clone := CloneNatIPv4(src)
	if clone.Enabled == nil || *clone.Enabled != false {
		t.Fatal("enabled pointer not preserved")
	}
	*clone.Enabled = true
	if NatIPv4Enabled(src) {
		t.Fatal("clone must not alias enabled pointer")
	}
	if !NatIPv4Enabled(clone) {
		t.Fatal("clone enabled should update independently")
	}
}
