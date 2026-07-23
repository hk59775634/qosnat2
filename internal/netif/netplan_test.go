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

func TestRenderNetplanGatewayRoutes(t *testing.T) {
	body, _, err := RenderNetplan(store.NetworkState{
		Ifaces: []store.IfaceConfig{
			{
				Device:  "ens18",
				IPv4:    []string{"103.127.237.22/30"},
				Up:      true,
				Gateway: "103.127.237.21",
			},
			{
				Device:        "ens20",
				IPv4:          []string{"109.244.68.67/24"},
				Up:            true,
				Gateway:       "109.244.68.1",
				PolicyRouting: true,
			},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	if !strings.Contains(s, "ens18:") || !strings.Contains(s, "via: 103.127.237.21") {
		t.Fatalf("ens18 main gateway missing: %s", s)
	}
	// ens20 策略路由口不得写入主表 default
	ens20Idx := strings.Index(s, "ens20:")
	if ens20Idx < 0 {
		t.Fatalf("missing ens20: %s", s)
	}
	ens20Block := s[ens20Idx:]
	if next := strings.Index(ens20Block[6:], "\n    "); next >= 0 {
		ens20Block = ens20Block[:6+next]
	}
	if strings.Contains(ens20Block, "via: 109.244.68.1") || strings.Contains(ens20Block, "to: default") {
		t.Fatalf("policy iface must not install main default: %s", ens20Block)
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

func TestIsNetplanManagedDevice(t *testing.T) {
	if !IsNetplanManagedDevice("ens18") {
		t.Fatal("ens18 should be manageable")
	}
	for _, bad := range []string{"lo", "ifb0", "veth0", "docker0", "br-abc", "qpe0", "qwp0", "CloudflareWARP"} {
		if IsNetplanManagedDevice(bad) {
			t.Fatalf("%s should be excluded", bad)
		}
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

