package ocserv

import (
	"strings"
	"testing"
)

func TestSystemdUnitContent(t *testing.T) {
	s := systemdUnitContent("/usr/local/sbin/ocserv", "/etc/ocserv")
	if !strings.Contains(s, "ExecStart=/usr/local/sbin/ocserv") {
		t.Fatalf("missing ExecStart: %s", s)
	}
	if !strings.Contains(s, "--config /etc/ocserv/ocserv.conf") {
		t.Fatalf("missing config path: %s", s)
	}
}
