package frr

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderManagedNativeFormat(t *testing.T) {
	body, err := RenderManaged([]store.RouteEntry{{
		Dest:    "10.0.0.0/8",
		Gateway: "192.168.1.1",
		Enabled: true,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(body, "configure terminal") {
		t.Fatalf("native config must not include vtysh shell commands: %s", body)
	}
	if !strings.Contains(body, "ip route 10.0.0.0/8 192.168.1.1") {
		t.Fatalf("missing route line: %s", body)
	}
}

func TestRenderManagedVTYApply(t *testing.T) {
	prev := "! gen\nip route 10.0.0.0/8 192.168.1.1\n"
	newNative := "! gen\nip route 10.0.0.0/8 192.168.1.2\n"
	script := renderManagedVTYApply(prev, newNative)
	for _, want := range []string{
		"configure terminal",
		"no ip route 10.0.0.0/8 192.168.1.1",
		"ip route 10.0.0.0/8 192.168.1.2",
		"write memory",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("missing %q in:\n%s", want, script)
		}
	}
}
