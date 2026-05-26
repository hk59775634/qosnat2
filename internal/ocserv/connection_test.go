package ocserv

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestBuildConnectionURL_port443_noCamo(t *testing.T) {
	o := store.DefaultOCServ()
	o.ManagedCertID = "c1"
	o.TCPPort = 443
	ci := BuildConnectionInfo(o, []store.ManagedCertificate{
		{ID: "c1", Domains: []string{"vpn.example.com"}, CertPath: "/nonexistent"},
	})
	if ci.URL != "https://vpn.example.com" {
		t.Fatalf("url=%q", ci.URL)
	}
	if ci.PortInURL {
		t.Fatal("port should be omitted for 443")
	}
}

func TestBuildConnectionURL_customPort_camouflage(t *testing.T) {
	o := store.DefaultOCServ()
	o.ManagedCertID = "c1"
	o.TCPPort = 8443
	o.Advanced.Camouflage = true
	o.Advanced.CamouflageSecret = "my-secret"
	ci := BuildConnectionInfo(o, []store.ManagedCertificate{
		{ID: "c1", Domains: []string{"vpn.example.com"}},
	})
	want := "https://vpn.example.com:8443/?my-secret"
	if ci.URL != want {
		t.Fatalf("got %q want %q", ci.URL, want)
	}
	if ci.CamouflageSecret != "my-secret" {
		t.Fatalf("secret=%q", ci.CamouflageSecret)
	}
}

func TestBuildConnectionURL_camouflageMissingSecret(t *testing.T) {
	o := store.DefaultOCServ()
	o.ManagedCertID = "c1"
	o.Advanced.Camouflage = true
	ci := BuildConnectionInfo(o, []store.ManagedCertificate{
		{ID: "c1", Domains: []string{"vpn.example.com"}},
	})
	if ci.Issue != "camouflage_secret_missing" {
		t.Fatalf("issue=%q", ci.Issue)
	}
	if ci.URL != "https://vpn.example.com" {
		t.Fatalf("url=%q", ci.URL)
	}
}
