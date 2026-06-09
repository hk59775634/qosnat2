package store

import "testing"

func TestOCServRadiusUsesGroupconfig(t *testing.T) {
	plain := DefaultOCServ()
	if OCServRadiusUsesGroupconfig(plain) {
		t.Fatal("plain auth")
	}
	radiusLocal := DefaultOCServ()
	radiusLocal.AuthMethod = OCServAuthRadius
	radiusLocal.Radius.GroupConfig = false
	if OCServRadiusUsesGroupconfig(radiusLocal) {
		t.Fatal("radius without groupconfig")
	}
	radiusGC := DefaultOCServ()
	radiusGC.AuthMethod = OCServAuthRadius
	radiusGC.Radius.GroupConfig = true
	if !OCServRadiusUsesGroupconfig(radiusGC) {
		t.Fatal("radius with groupconfig")
	}
}
