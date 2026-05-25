package ocserv

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestParseRadiusServerHost(t *testing.T) {
	cases := []struct {
		in, wantHost string
		wantPort     int
	}{
		{"103.6.4.138", "103.6.4.138", 0},
		{"103.6.4.138:1812", "103.6.4.138", 1812},
		{"[2001:db8::1]", "2001:db8::1", 0},
		{"[2001:db8::1]:1812", "2001:db8::1", 1812},
	}
	for _, c := range cases {
		host, port, err := parseRadiusServerHost(c.in)
		if err != nil || host != c.wantHost || port != c.wantPort {
			t.Fatalf("%q => %q,%d,%v want %q,%d", c.in, host, port, err, c.wantHost, c.wantPort)
		}
	}
}

func TestRadcliServersLineNoPort(t *testing.T) {
	host, _, err := parseRadiusServerHost("103.6.4.138:1812")
	if err != nil {
		t.Fatal(err)
	}
	line := radcliServersLine(host, "secret")
	if strings.Contains(strings.Split(line, "\t")[0], ":") {
		t.Fatalf("servers first field must be host only, got %q", line)
	}
}

func radcliServersLine(host, secret string) string {
	return host + "\t" + secret + "\t3\n"
}

func TestNormalizeRadiusConfigStripsPort(t *testing.T) {
	o := store.DefaultOCServ()
	o.AuthMethod = store.OCServAuthRadius
	o.Radius.Server = "10.0.0.5:1812"
	o.Radius.AuthPort = 1812
	o.Radius.Secret = "s"
	if err := NormalizeRadiusConfig(&o); err != nil {
		t.Fatal(err)
	}
	if o.Radius.Server != "10.0.0.5" {
		t.Fatalf("server=%q", o.Radius.Server)
	}
}

func TestRadcliDictionaryOcservAndCiscoASA(t *testing.T) {
	for _, sub := range []string{
		"ATTRIBUTE	Calling-Station-Id	31	string",
		"ATTRIBUTE	Acct-Status-Type	40	integer",
		"VENDOR Cisco-ASA 3076",
		"ATTRIBUTE	ASA-Address-Pools",
	} {
		if !strings.Contains(radcliDictionary, sub) {
			t.Fatalf("radcli dictionary missing %q", sub)
		}
	}
}
