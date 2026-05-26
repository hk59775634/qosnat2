package api

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
)

func wireGuardPublic(c store.WireGuardState) map[string]any {
	peers := make([]map[string]any, 0, len(c.Peers))
	for _, p := range c.Peers {
		peers = append(peers, map[string]any{
			"name":                  p.Name,
			"public_key":            p.PublicKey,
			"allowed_ips":           p.AllowedIPs,
			"endpoint":              p.Endpoint,
			"persistent_keepalive":  p.PersistentKeepalive,
			"rate":                  p.Rate,
			"private_key_set":       strings.TrimSpace(p.PrivateKey) != "",
			"preshared_key_set":     strings.TrimSpace(p.PresharedKey) != "",
		})
	}
	return map[string]any{
		"enabled":                c.Enabled,
		"interface":              c.Interface,
		"listen_port":            c.ListenPort,
		"address":                c.Address,
		"public_key":             c.PublicKey,
		"dns":                    c.DNS,
		"server_endpoint":        c.ServerEndpoint,
		"peers":                  peers,
		"server_private_key_set": strings.TrimSpace(c.PrivateKey) != "",
	}
}

func mergeWireGuardSecrets(body *store.WireGuardState, prev store.WireGuardState) {
	if strings.TrimSpace(body.PrivateKey) == "" {
		body.PrivateKey = prev.PrivateKey
	}
	prevByName := map[string]store.WGPeer{}
	for _, p := range prev.Peers {
		prevByName[p.Name] = p
	}
	for i := range body.Peers {
		pp, ok := prevByName[body.Peers[i].Name]
		if !ok {
			continue
		}
		if strings.TrimSpace(body.Peers[i].PrivateKey) == "" {
			body.Peers[i].PrivateKey = pp.PrivateKey
		}
		if strings.TrimSpace(body.Peers[i].PresharedKey) == "" {
			body.Peers[i].PresharedKey = pp.PresharedKey
		}
	}
}
