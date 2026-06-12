package snmpd

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderConf(t *testing.T) {
	body := RenderConf(store.SNMPState{
		Enabled:         true,
		Port:            161,
		SysLocation:     "rack-1",
		ROCommunity:     "secret",
		AllowedNetworks: []string{"192.168.1.0/24"},
	})
	for _, want := range []string{
		"sysLocation rack-1",
		"agentAddress udp:0.0.0.0:161",
		"rocommunity secret 192.168.1.0/24",
		"view systemonly",
		".1.3.6.1.2.1.2",
		".1.3.6.1.2.1.31",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}

func TestRenderConfDisabled(t *testing.T) {
	body := RenderConf(store.SNMPState{Enabled: false})
	if !strings.Contains(body, "disabled") || strings.Contains(body, "rocommunity") {
		t.Fatalf("disabled conf should not expose community:\n%s", body)
	}
}
