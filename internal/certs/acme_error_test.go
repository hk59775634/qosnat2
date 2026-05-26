package certs

import (
	"errors"
	"testing"
)

func TestClassifyACMEErrorDNS(t *testing.T) {
	err := errors.New(`acme: error: 400 :: urn:ietf:params:acme:error:dns :: no valid A records found for vpn.example.com`)
	info := ClassifyACMEError(err)
	if !info.IsDNS || !info.PauseAutoRenew {
		t.Fatalf("expected dns pause, got %+v", info)
	}
}

func TestClassifyACMEErrorOther(t *testing.T) {
	err := errors.New("connection refused")
	info := ClassifyACMEError(err)
	if info.PauseAutoRenew {
		t.Fatalf("expected no pause for generic error")
	}
}
