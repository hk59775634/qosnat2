package acme

import (
	"testing"

	"github.com/go-acme/lego/v4/registration"
)

func TestRegistrationMatches(t *testing.T) {
	prod := &registration.Resource{URI: "https://acme-v02.api.letsencrypt.org/acme/acct/1"}
	stg := &registration.Resource{URI: "https://acme-staging-v02.api.letsencrypt.org/acme/acct/1"}
	if !registrationMatches(prod, false) || registrationMatches(prod, true) {
		t.Fatal("prod mismatch")
	}
	if !registrationMatches(stg, true) || registrationMatches(stg, false) {
		t.Fatal("staging mismatch")
	}
}

func TestAccountRegPath(t *testing.T) {
	if accountRegPath(true) == accountRegPath(false) {
		t.Fatal("staging and prod paths should differ")
	}
}
