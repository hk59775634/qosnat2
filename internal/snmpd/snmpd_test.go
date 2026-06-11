package snmpd

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderConf(t *testing.T) {
	body := RenderConf(store.SNMPState{
		Enabled:             true,
		Port:                161,
		ListenLocalhostOnly: false,
		SysLocation:         "rack-1",
		ROCommunity:         "secret",
		AllowedNetworks:     []string{"10.0.0.0/8"},
	})
	for _, want := range []string{
		"sysLocation rack-1",
		"agentAddress udp:0.0.0.0:161",
		"rocommunity secret 10.0.0.0/8",
		"view systemonly",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing %q in:\n%s", want, body)
		}
	}
}
