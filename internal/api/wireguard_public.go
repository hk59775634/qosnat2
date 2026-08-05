package api

import (
	"fmt"
	"strings"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
	wgusertraffic "github.com/hk59775634/qosnat2/internal/wg/usertraffic"
)

func wireGuardInstancePublic(inst store.WireGuardInstance) map[string]any {
	tr := wgusertraffic.DefaultStore()
	peers := make([]map[string]any, 0, len(inst.Peers))
	for _, p := range inst.Peers {
		rx, tx := tr.TotalBytes(store.WgPeerTrafficKey(inst.ID, p.Name))
		peers = append(peers, map[string]any{
			"name":                 p.Name,
			"public_key":           p.PublicKey,
			"allowed_ips":          p.AllowedIPs,
			"endpoint":             p.Endpoint,
			"persistent_keepalive": p.PersistentKeepalive,
			"rate":                 p.Rate,
			"route_allowed_ips":    store.PeerRouteAllowedIPs(p),
			"private_key_set":      strings.TrimSpace(p.PrivateKey) != "",
			"preshared_key_set":    strings.TrimSpace(p.PresharedKey) != "",
			"total_rx_bytes":       rx,
			"total_tx_bytes":       tx,
			"total_bytes":          rx + tx,
		})
	}
	return map[string]any{
		"id":                     inst.ID,
		"name":                   inst.Name,
		"mode":                   inst.Mode,
		"enabled":                inst.Enabled,
		"interface":              inst.Interface,
		"listen_port":            inst.ListenPort,
		"address":                inst.Address,
		"public_key":             inst.PublicKey,
		"dns":                    inst.DNS,
		"server_endpoint":        inst.ServerEndpoint,
		"peers":                  peers,
		"server_private_key_set": strings.TrimSpace(inst.PrivateKey) != "",
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

func mergeWireGuardInstanceSecrets(body *store.WireGuardInstance, prev store.WireGuardInstance) {
	mergeWireGuardSecrets(&body.WireGuardState, prev.WireGuardState)
}

// deriveWireGuardPublicKey 在已有私钥时重算公钥（便于粘贴恢复旧配置）。
func deriveWireGuardPublicKey(st *store.WireGuardState) error {
	if st == nil {
		return nil
	}
	priv := strings.TrimSpace(st.PrivateKey)
	if priv == "" {
		return nil
	}
	pub, err := wg.PublicKeyFromPrivate(priv)
	if err != nil {
		return fmt.Errorf("private_key: %w", err)
	}
	st.PublicKey = pub
	return nil
}
