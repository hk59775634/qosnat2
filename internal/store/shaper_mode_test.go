package store

import "testing"

func TestEffectiveShaperMode(t *testing.T) {
	if EffectiveShaperMode(ShaperState{}) != ShaperModeEDT {
		t.Fatal("empty mode should default to edt")
	}
	if EffectiveShaperMode(ShaperState{Mode: "edt"}) != ShaperModeEDT {
		t.Fatal("edt")
	}
	if EffectiveShaperMode(ShaperState{Mode: "htb"}) != ShaperModeHTB {
		t.Fatal("htb")
	}
}
