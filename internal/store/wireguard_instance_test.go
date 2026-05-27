package store

import (
	"encoding/json"
	"testing"
)

func TestMigrateLegacyWireGuardJSON(t *testing.T) {
	raw := `{
		"wireguard": {
			"enabled": true,
			"interface": "wg0",
			"listen_port": 51820,
			"address": "10.99.0.1/24",
			"peers": []
		},
		"ocserv": {"enabled": false}
	}`
	var v VPNState
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	if v.LegacyWireGuard == nil {
		t.Fatal("expected legacy wireguard")
	}
	MigrateLegacyWireGuardToInstances(&v)
	if len(v.WireGuards) != 1 {
		t.Fatalf("instances: %+v", v.WireGuards)
	}
	if v.WireGuards[0].Address != "10.99.0.1/24" || !v.WireGuards[0].Enabled {
		t.Fatalf("migrated: %+v", v.WireGuards[0])
	}
	if v.LegacyWireGuard != nil {
		t.Fatal("legacy pointer should be cleared after migrate")
	}
}
