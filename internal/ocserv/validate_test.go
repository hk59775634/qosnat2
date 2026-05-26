package ocserv

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestValidateStateCiscoSvcRequiresUDP443(t *testing.T) {
	o := store.DefaultOCServ()
	o.Advanced.CiscoSvcCompat = true
	o.Advanced.Udp = true
	o.UDPPort = 444
	err := ValidateState(o)
	if err == nil || !strings.Contains(err.Error(), "udp-port = 443") {
		t.Fatalf("expected udp-port error, got %v", err)
	}
	o.UDPPort = 443
	if err := ValidateState(o); err != nil {
		t.Fatalf("expected ok: %v", err)
	}
}

func TestValidateStateCiscoSvcVhost(t *testing.T) {
	o := store.DefaultOCServ()
	o.Advanced.CiscoSvcCompat = false
	o.UDPPort = 8443
	o.Vhosts = []store.OCServVhost{{Domain: "vpn.example.com", CiscoSvcCompat: true}}
	if err := ValidateState(o); err == nil {
		t.Fatal("expected error when vhost enables cisco-svc without udp 443")
	}
}
