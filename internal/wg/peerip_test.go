package wg

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestPeerRateShapeIP_ClientUsesLocalTunnel(t *testing.T) {
	inst := store.WireGuardInstance{
		Mode: store.WGModeClient,
		WireGuardState: store.WireGuardState{
			Address: "100.64.0.201/32",
		},
	}
	p := store.WGPeer{
		AllowedIPs: []string{"0.0.0.0/0", "::/0"},
	}
	if got := PeerRateShapeIP(inst, p); got != "100.64.0.201" {
		t.Fatalf("client shape IP: got %q want 100.64.0.201", got)
	}
}

func TestPeerRateShapeIP_ServerUsesAllowedIPs(t *testing.T) {
	inst := store.WireGuardInstance{
		Mode: store.WGModeServer,
		WireGuardState: store.WireGuardState{
			Address: "10.200.0.1/24",
		},
	}
	p := store.WGPeer{
		AllowedIPs: []string{"10.200.0.55/32"},
	}
	if got := PeerRateShapeIP(inst, p); got != "10.200.0.55" {
		t.Fatalf("server shape IP: got %q want 10.200.0.55", got)
	}
}
