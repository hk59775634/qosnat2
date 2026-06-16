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
	if !strings.Contains(conf, "select-group-by-url = true") {
		t.Fatalf("missing select-group-by-url: %s", conf)
	}
	if strings.Contains(conf, "plain[passwd") {
		t.Fatal("should not use plain auth")
	}
	if strings.Contains(conf, "\nconfig-per-group = ") {
		t.Fatalf("radius groupconfig must comment out config-per-group:\n%s", conf)
	}
	if !strings.Contains(conf, "# config-per-group = /etc/ocserv/config-per-group/") {
		t.Fatalf("missing commented config-per-group in radius groupconfig mode:\n%s", conf)
	}
	if !strings.Contains(conf, "# default-group-config = /etc/ocserv/defaults/group.conf") {
		t.Fatalf("missing commented default-group-config in radius groupconfig mode:\n%s", conf)
	}
}

func TestRenderConfRadiusEmptyPool(t *testing.T) {
	o := store.DefaultOCServ()
	o.AuthMethod = store.OCServAuthRadius
	o.Radius = store.OCServRadius{
		Server:      "10.0.0.1",
		AuthPort:    1812,
		Secret:      "s3cret",
		GroupConfig: true,
	}
	o.IPv4Network = ""
	o.IPv4Netmask = ""
	if err := store.NormalizeOCServ(&o); err != nil {
		t.Fatal(err)
	}
	conf := RenderConf(o, nil)
	if strings.Contains(conf, "ipv4-network = ") {
		t.Fatalf("radius without local pool must omit ipv4-network:\n%s", conf)
	}
}

func TestRenderConfRadiusLocalPool(t *testing.T) {
	o := store.DefaultOCServ()
	o.AuthMethod = store.OCServAuthRadius
	o.Radius = store.OCServRadius{
		Server:   "10.0.0.1",
		AuthPort: 1812,
		Secret:   "s3cret",
	}
	o.IPv4Network = "10.9.0.0"
	o.IPv4Netmask = "255.255.255.0"
	conf := RenderConf(o, nil)
	if !strings.Contains(conf, "ipv4-network = 10.9.0.0") {
		t.Fatalf("missing local pool:\n%s", conf)
	}
}

func TestRenderConfRadiusNoGroupconfig(t *testing.T) {
	o := store.DefaultOCServ()
	o.AuthMethod = store.OCServAuthRadius
	o.Radius = store.OCServRadius{
		Server:      "10.0.0.1",
		AuthPort:    1812,
		Secret:      "s3cret",
		GroupConfig: false,
	}
	o.Groups = []store.OCServGroup{
		{Name: "hk", Label: "香港"},
		{Name: "us", Label: "美国"},
	}
	conf := RenderConf(o, nil)
	if !strings.Contains(conf, `auth = "radius[config=/etc/radcli/radiusclient.conf]"`) {
		t.Fatalf("missing radius auth without groupconfig: %s", conf)
	}
	if strings.Contains(conf, "groupconfig=true") {
		t.Fatal("should not include groupconfig")
	}
	if !strings.Contains(conf, "config-per-group = /etc/ocserv/config-per-group/") {
		t.Fatalf("missing config-per-group:\n%s", conf)
	}
	if !strings.Contains(conf, "auto-select-group = false") {
		t.Fatalf("missing auto-select-group false:\n%s", conf)
	}
	if !strings.Contains(conf, "select-group = hk[香港]") || !strings.Contains(conf, "select-group = us[美国]") {
		t.Fatalf("missing select-group lines:\n%s", conf)
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

func TestRenderConfSkipsDisabledVhost(t *testing.T) {
	o := store.DefaultOCServ()
	o.Vhosts = []store.OCServVhost{
		{Enabled: false, Domain: "disabled.example.com"},
		{Enabled: true, Domain: "active.example.com"},
	}
	conf := RenderConf(o, nil)
	if strings.Contains(conf, "[vhost:disabled.example.com]") {
		t.Fatalf("disabled vhost must not be rendered:\n%s", conf)
	}
	if !strings.Contains(conf, "[vhost:active.example.com]") {
		t.Fatalf("active vhost missing:\n%s", conf)
	}
}
