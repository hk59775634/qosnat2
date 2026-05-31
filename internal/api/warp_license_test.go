package api

import (
	"errors"
	"testing"
)

func TestIsWarpLicenseError(t *testing.T) {
	if !isWarpLicenseError(errors.New("warp license: invalid key")) {
		t.Fatal("expected license error")
	}
	if isWarpLicenseError(errors.New("warp mode: failed")) {
		t.Fatal("unexpected license match")
	}
	if isWarpLicenseError(nil) {
		t.Fatal("nil should be false")
	}
}
