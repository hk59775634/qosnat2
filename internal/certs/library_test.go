package certs

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestFindBestCertForDomain(t *testing.T) {
	all := []store.ManagedCertificate{
		{ID: "a", Type: store.CertTypeACME, Domains: []string{"vpn.example.com"}, CertPath: "/nonexistent-a"},
		{ID: "b", Type: store.CertTypeACME, Domains: []string{"vpn.example.com"}, CertPath: "/nonexistent-b"},
	}
	_, ok := FindBestCertForDomain(all, "vpn.example.com", "a")
	if !ok {
		t.Fatal("expected match")
	}
}
