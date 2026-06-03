package netif

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRenderNetplan(t *testing.T) {
	body, down, err := RenderNetplan(store.NetworkState{
		Ifaces: []store.IfaceConfig{
			{Device: "ens19", IPv4: []string{"100.64.0.249/24"}, Up: true},
		},
		VLANs: []store.VLANIface{
			{Parent: "ens19", VID: 100, IPv4: []string{"192.168.100.1/24"}, Up: false},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "ens19:") || !strings.Contains(s, "dhcp4: no") {
		t.Fatalf("missing ethernet: %s", s)
	}
	if !strings.Contains(s, "ens19.100:") || !strings.Contains(s, "id: 100") {
		t.Fatalf("missing vlan: %s", s)
	}
	if len(down) != 1 || down[0] != "ens19.100" {
		t.Fatalf("down list: %v", down)
	}
}

func TestRenderNetplanEmpty(t *testing.T) {
	body, _, err := RenderNetplan(store.NetworkState{})
	if err != nil {
		t.Fatal(err)
	}
	if len(body) != 0 {
		t.Fatalf("expected empty, got %q", body)
	}
}

func TestApplyNetplanEmptySkipsGenerate(t *testing.T) {
	if _, err := exec.LookPath("netplan"); err != nil {
		t.Skip("netplan not installed")
	}
	applied, err := ApplyNetplan(store.NetworkState{})
	if err != nil {
		t.Fatal(err)
	}
	if applied {
		t.Fatal("expected no netplan apply for empty state")
	}
}
