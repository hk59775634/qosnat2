package api

import (
	"bytes"
	"net/http"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

// PUT /vpn/wireguard/instances/{id} 的 config 含只读字段；PUT 必须能接受同一形状
func TestWireGuardInstancePUTDecodesPublicShape(t *testing.T) {
	raw := `{
		"id": "default",
		"name": "default",
		"mode": "server",
		"enabled": false,
		"interface": "wg0",
		"listen_port": 51820,
		"address": "10.200.0.1/24",
		"private_key": "",
		"public_key": "dGVzdA==",
		"dns": [],
		"server_endpoint": "",
		"peers": [{
			"name": "p1",
			"public_key": "cGsx",
			"allowed_ips": ["10.200.0.2/32"],
			"private_key_set": true,
			"preshared_key_set": false,
			"total_rx_bytes": 99,
			"total_tx_bytes": 88,
			"total_bytes": 187,
			"rate": {"down": "", "up": ""}
		}],
		"server_private_key_set": true
	}`
	reqStrict, _ := http.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte(raw)))
	var strict store.WireGuardInstance
	if err := readJSON(reqStrict, &strict); err == nil {
		t.Fatal("readJSON with DisallowUnknownFields should reject server_private_key_set / total_*")
	}

	reqRel, _ := http.NewRequest(http.MethodPut, "/", bytes.NewReader([]byte(raw)))
	var body store.WireGuardInstance
	if err := readJSONRelaxed(reqRel, &body); err != nil {
		t.Fatal(err)
	}
	if body.ID != "default" || body.Interface != "wg0" || len(body.Peers) != 1 {
		t.Fatalf("decode: %+v", body)
	}
	if body.Peers[0].Name != "p1" || body.Peers[0].PublicKey != "cGsx" {
		t.Fatalf("peer: %+v", body.Peers[0])
	}
}
