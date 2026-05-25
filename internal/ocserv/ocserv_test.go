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
	conf := RenderConf(o)
	if !strings.Contains(conf, `auth = "radius[config=/etc/radcli/radiusclient.conf,groupconfig=true]"`) {
		t.Fatalf("missing radius auth: %s", conf)
	}
	if !strings.Contains(conf, `acct = "radius[config=/etc/radcli/radiusclient.conf]"`) {
		t.Fatalf("missing acct: %s", conf)
	}
	if !strings.Contains(conf, "stats-report-time = 360") {
		t.Fatalf("missing stats-report-time: %s", conf)
	}
	if strings.Contains(conf, "plain[passwd") {
		t.Fatal("should not use plain auth")
	}
}

func TestRenderConfPlain(t *testing.T) {
	o := store.DefaultOCServ()
	conf := RenderConf(o)
	if !strings.Contains(conf, `plain[passwd=`) {
		t.Fatalf("%s", conf)
	}
}
