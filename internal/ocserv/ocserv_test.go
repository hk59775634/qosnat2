package ocserv

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderConfRadius(t *testing.T) {
	o := store.DefaultOCServ()
	o.AuthMethod = store.OCServAuthRadius
	o.Radius = store.OCServRadius{
		Server:      "10.0.0.1",
		AuthPort:    1812,
		Secret:      "s3cret",
		GroupConfig: true,
		AcctEnabled: true,
	}
	conf := RenderConf(o, nil)
	if !strings.Contains(conf, `auth = "radius[config=/etc/radcli/radiusclient.conf,groupconfig=true]"`) {
		t.Fatalf("missing radius auth: %s", conf)
	}
	if !strings.Contains(conf, `acct = "radius[config=/etc/radcli/radiusclient.conf]"`) {
		t.Fatalf("missing acct: %s", conf)
	}
	if !strings.Contains(conf, "stats-report-time = 360") {
		t.Fatalf("missing stats-report-time: %s", conf)
	}
	if !strings.Contains(conf, "try-mtu-discovery = true") {
		t.Fatalf("missing advanced: %s", conf)
	}
	if strings.Contains(conf, "plain[passwd") {
		t.Fatal("should not use plain auth")
	}
	if strings.Contains(conf, "config-per-group") {
		t.Fatalf("radius must not set config-per-group (conflicts with radius supplemental config):\n%s", conf)
	}
	if strings.Contains(conf, "config-per-user") {
		t.Fatalf("radius must not set config-per-user:\n%s", conf)
	}
}

func TestRenderConfAdvancedOff(t *testing.T) {
	o := store.DefaultOCServ()
	o.Advanced = store.DefaultOCServAdvanced()
	o.Advanced.Udp = false
	o.Advanced.DtlsLegacy = false
	conf := RenderConf(o, nil)
	if strings.Contains(conf, "udp-port") {
		t.Fatal("udp should be disabled")
	}
	if strings.Contains(conf, "dtls-legacy = true") {
		t.Fatal("dtls-legacy should be false")
	}
}

func TestRenderConfExtras(t *testing.T) {
	o := store.DefaultOCServ()
	o.NoRoutes = []string{"192.168.5.0/255.255.255.0"}
	o.UseQoSnatTLS = false
	o.ServerCertPath = "/etc/ocserv/certs/server-cert.pem"
	o.Advanced.Camouflage = true
	o.Advanced.CamouflageSecret = "secret"
	o.Advanced.CamouflageRealm = "Restricted"
	o.Advanced.RxDataPerSec = 1250000
	o.Advanced.RateLimitMs = 100
	conf := RenderConf(o, nil)
	for _, want := range []string{
		"no-route = 192.168.5.0/255.255.255.0",
		"server-cert = /etc/ocserv/certs/server-cert.pem",
		"camouflage = true",
		"rx-data-per-sec = 1250000",
		"rate-limit-ms = 100",
		"cisco-svc-client-compat",
	} {
		if !strings.Contains(conf, want) {
			t.Fatalf("missing %q in:\n%s", want, conf)
		}
	}
}

func TestRenderConfManagedCert(t *testing.T) {
	o := store.DefaultOCServ()
	o.UseQoSnatTLS = false
	o.ManagedCertID = "cert-abc"
	certs := []store.ManagedCertificate{{
		ID:       "cert-abc",
		CertPath: "/var/lib/qosnat2/certs/cert-abc/fullchain.pem",
		KeyPath:  "/var/lib/qosnat2/certs/cert-abc/privkey.pem",
	}}
	conf := RenderConf(o, certs)
	if !strings.Contains(conf, "server-cert = /var/lib/qosnat2/certs/cert-abc/fullchain.pem") {
		t.Fatalf("expected managed cert path: %s", conf)
	}
}

func TestRenderConfPlain(t *testing.T) {
	o := store.DefaultOCServ()
	conf := RenderConf(o, nil)
	if !strings.Contains(conf, `plain[passwd=`) {
		t.Fatalf("%s", conf)
	}
}
