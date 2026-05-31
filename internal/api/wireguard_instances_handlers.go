package api

import (
	"log"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
	wgusertraffic "github.com/hk59775634/qosnat2/internal/wg/usertraffic"
)

const wireGuardInstancesPrefix = "/api/v1/vpn/wireguard/instances"

var wgPeerOnlineHandshakeWindow = 10 * time.Minute

func (srv *Server) handleWireGuardInstancesRoot(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		out := make([]map[string]any, 0, len(st.VPN.WireGuards))
		for i := range st.VPN.WireGuards {
			inst := st.VPN.WireGuards[i]
			iface := strings.TrimSpace(inst.Interface)
			if iface == "" {
				iface = "wg0"
			}
			out = append(out, map[string]any{
				"id":     inst.ID,
				"name":   inst.Name,
				"mode":   inst.Mode,
				"enabled": inst.Enabled,
				"interface": iface,
				"listen_port": inst.ListenPort,
				"status": wg.ShowStatus(iface),
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"instances": out})
	case http.MethodPost:
		var body struct {
			ID   string             `json:"id"`
			Name string             `json:"name"`
			Mode store.WireGuardMode `json:"mode"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		id := strings.TrimSpace(body.ID)
		if id == "" {
			id = store.NewWireGuardInstanceID()
		}
		mode := body.Mode
		if mode != store.WGModeClient {
			mode = store.WGModeServer
		}
		name := strings.TrimSpace(body.Name)
		if name == "" {
			name = id
		}
		var errStr string
		_ = srv.store.Update(func(s *store.State) {
			if _, ok := store.FindWireGuardInstance(s.VPN.WireGuards, id); ok {
				errStr = "id already exists"
				return
			}
			inst := store.WireGuardInstance{ID: id, Name: name, Mode: mode}
			store.NormalizeWireGuardInstance(&inst)
			n := len(s.VPN.WireGuards)
			if n > 0 {
				inst.Interface = fmt.Sprintf("wg%d", n)
				if inst.Mode == store.WGModeServer {
					inst.ListenPort = 51820 + n
				}
				inst.Address = fmt.Sprintf("10.200.%d.1/24", n)
			}
			s.VPN.WireGuards = append(s.VPN.WireGuards, inst)
			if msg := validateWireGuardInstancesUnique(s.VPN.WireGuards); msg != "" {
				s.VPN.WireGuards = s.VPN.WireGuards[:len(s.VPN.WireGuards)-1]
				errStr = msg
			}
		})
		if errStr != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": errStr})
			return
		}
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "id": id})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWireGuardInstancesSubtree(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, wireGuardInstancesPrefix+"/")
	rest = strings.Trim(rest, "/")
	parts := strings.Split(rest, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.NotFound(w, r)
		return
	}
	id := parts[0]

	if len(parts) == 1 {
		srv.handleWireGuardInstanceOne(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "apply" {
		srv.handleWireGuardInstanceApply(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "keys" {
		srv.handleWireGuardInstanceKeys(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "peer-genkey" {
		srv.handleWireGuardInstancePeerGenkey(w, r, id)
		return
	}
	if len(parts) == 2 && parts[1] == "peers" {
		srv.handleWireGuardInstancePeers(w, r, id)
		return
	}
	if len(parts) == 3 && parts[1] == "peers" && parts[2] == "traffic" {
		srv.handleWireGuardInstancePeerTraffic(w, r, id)
		return
	}
	if len(parts) == 4 && parts[1] == "peers" && parts[3] == "conf" {
		srv.handleWireGuardInstancePeerConf(w, r, id, parts[2])
		return
	}
	http.NotFound(w, r)
}

func (srv *Server) handleWireGuardInstanceOne(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		idx, ok := store.FindWireGuardInstance(st.VPN.WireGuards, id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
			return
		}
		inst := st.VPN.WireGuards[idx]
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"config": wireGuardInstancePublic(inst),
			"status": wg.ShowStatus(iface),
		})
	case http.MethodPut:
		var body store.WireGuardInstance
		if err := readJSONRelaxed(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		body.ID = id
		var errStr string
		_ = srv.store.Update(func(s *store.State) {
			idx, ok := store.FindWireGuardInstance(s.VPN.WireGuards, id)
			if !ok {
				errStr = "instance not found"
				return
			}
			prev := s.VPN.WireGuards[idx]
			mergeWireGuardInstanceSecrets(&body, prev)
			store.NormalizeWireGuardInstance(&body)
			if err := store.ValidateWireGuardInstancePatch(body); err != nil {
				errStr = err.Error()
				return
			}
			list := append([]store.WireGuardInstance(nil), s.VPN.WireGuards[:idx]...)
			list = append(list, body)
			list = append(list, s.VPN.WireGuards[idx+1:]...)
			if msg := validateWireGuardInstancesUnique(list); msg != "" {
				errStr = msg
				return
			}
			s.VPN.WireGuards = list
		})
		if errStr != "" {
			if errStr == "instance not found" {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": errStr})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": errStr})
			return
		}
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		srv.setupWGShaper()
		nftWarn := srv.tryReloadNft()
		if nftWarn != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nft_warning": nftWarn})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		var errStr string
		_ = srv.store.Update(func(s *store.State) {
			idx, ok := store.FindWireGuardInstance(s.VPN.WireGuards, id)
			if !ok {
				errStr = "instance not found"
				return
			}
			if len(s.VPN.WireGuards) <= 1 {
				errStr = "cannot delete last wireguard instance"
				return
			}
			removed := s.VPN.WireGuards[idx]
			s.VPN.WireGuards = append(s.VPN.WireGuards[:idx], s.VPN.WireGuards[idx+1:]...)
			_ = removed
		})
		if errStr != "" {
			code := http.StatusBadRequest
			if errStr == "instance not found" {
				code = http.StatusNotFound
			}
			writeJSON(w, code, map[string]string{"error": errStr})
			return
		}
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		srv.setupWGShaper()
		nftWarn := srv.tryReloadNft()
		if nftWarn != "" {
			writeJSON(w, http.StatusOK, map[string]any{"ok": true, "nft_warning": nftWarn})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWireGuardInstanceKeys(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kp, err := wg.GenKeyPair()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	var errStr string
	_ = srv.store.Update(func(s *store.State) {
		idx, ok := store.FindWireGuardInstance(s.VPN.WireGuards, id)
		if !ok {
			errStr = "instance not found"
			return
		}
		s.VPN.WireGuards[idx].PrivateKey = kp.Private
		s.VPN.WireGuards[idx].PublicKey = kp.Public
	})
	if errStr != "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": errStr})
		return
	}
	if !srv.persistState(w) {
		return
	}
	writeJSON(w, http.StatusOK, kp)
}

func (srv *Server) handleWireGuardInstancePeerGenkey(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	if _, ok := store.FindWireGuardInstance(st.VPN.WireGuards, id); !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
		return
	}
	kp, err := wg.GenKeyPair()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, kp)
}

func (srv *Server) handleWireGuardInstanceApply(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	idx, ok := store.FindWireGuardInstance(st.VPN.WireGuards, id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
		return
	}
	inst := st.VPN.WireGuards[idx]
	up := inst.Enabled
	if err := wg.Apply(inst.WireGuardState, up); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	srv.setupWGShaper()
	nftWarn := srv.tryReloadNft()
	resp := map[string]any{"ok": true, "up": up}
	if nftWarn != "" {
		resp["nft_warning"] = nftWarn
	}
	writeJSON(w, http.StatusOK, resp)
}

func (srv *Server) handleWireGuardInstancePeers(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		st := srv.store.Get()
		idx, ok := store.FindWireGuardInstance(st.VPN.WireGuards, id)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
			return
		}
		writeJSON(w, http.StatusOK, st.VPN.WireGuards[idx].Peers)
	case http.MethodPost:
		var p store.WGPeer
		if err := readJSONRelaxed(r, &p); err != nil || p.Name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		p.PrivateKey = strings.TrimSpace(p.PrivateKey)
		p.PublicKey = strings.TrimSpace(p.PublicKey)
		if p.PublicKey == "" && p.PrivateKey != "" {
			pub, err := wg.PublicKeyFromPrivate(p.PrivateKey)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid private key: " + err.Error()})
				return
			}
			p.PublicKey = pub
		}
		if p.PrivateKey == "" && p.PublicKey == "" {
			kp, err := wg.GenKeyPair()
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
			p.PrivateKey = kp.Private
			p.PublicKey = kp.Public
		}
		if p.PublicKey == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "public_key required (or provide private_key)"})
			return
		}
		if len(p.AllowedIPs) == 0 {
			p.AllowedIPs = []string{"10.200.0.10/32"}
		}
		var errStr string
		_ = srv.store.Update(func(s *store.State) {
			idx, ok := store.FindWireGuardInstance(s.VPN.WireGuards, id)
			if !ok {
				errStr = "instance not found"
				return
			}
			peers := s.VPN.WireGuards[idx].Peers
			var out []store.WGPeer
			replaced := false
			for _, e := range peers {
				if e.Name == p.Name {
					out = append(out, p)
					replaced = true
				} else {
					out = append(out, e)
				}
			}
			if !replaced {
				out = append(out, p)
			}
			s.VPN.WireGuards[idx].Peers = out
		})
		if errStr != "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": errStr})
			return
		}
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		srv.syncWGPeerRates()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		var removed *store.WGPeer
		var instSnapshot store.WireGuardInstance
		var errStr string
		_ = srv.store.Update(func(s *store.State) {
			idx, ok := store.FindWireGuardInstance(s.VPN.WireGuards, id)
			if !ok {
				errStr = "instance not found"
				return
			}
			instSnapshot = s.VPN.WireGuards[idx]
			var out []store.WGPeer
			for _, e := range s.VPN.WireGuards[idx].Peers {
				if e.Name == name {
					cp := e
					removed = &cp
					continue
				}
				out = append(out, e)
			}
			s.VPN.WireGuards[idx].Peers = out
		})
		if errStr != "" {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": errStr})
			return
		}
		if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
		if removed != nil {
			srv.removeWGPeerShaper(instSnapshot, *removed)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWireGuardInstancePeerConf(w http.ResponseWriter, r *http.Request, id, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	idx, ok := store.FindWireGuardInstance(st.VPN.WireGuards, id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
		return
	}
	inst := st.VPN.WireGuards[idx]
	var peer *store.WGPeer
	for i := range inst.Peers {
		if inst.Peers[i].Name == name {
			peer = &inst.Peers[i]
			break
		}
	}
	if peer == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "peer not found"})
		return
	}
	ep := inst.ServerEndpoint
	if ep == "" && srv.env.DevWAN != "" {
		ep = fmt.Sprintf("<WAN_IP>:%d", inst.ListenPort)
	}
	conf := wg.ClientConf(inst.WireGuardState, *peer, ep)
	fn, err := safeAttachmentFilename(name)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+fn+".conf")
	w.Write([]byte(conf))
}

func (srv *Server) handleWireGuardInstancePeerTraffic(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := strings.TrimSpace(r.URL.Query().Get("name"))
	if name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
		return
	}
	period := strings.TrimSpace(r.URL.Query().Get("period"))
	if period == "" {
		period = "7d"
	}
	switch period {
	case "24h", "7d", "30d", "365d", "1y":
	default:
		period = "7d"
	}
	if period == "1y" {
		period = "365d"
	}

	st := srv.store.Get()
	idx, ok := store.FindWireGuardInstance(st.VPN.WireGuards, id)
	if !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "instance not found"})
		return
	}
	inst := st.VPN.WireGuards[idx]
	var pub string
	for _, p := range inst.Peers {
		if p.Name == name {
			pub = strings.TrimSpace(p.PublicKey)
			break
		}
	}
	if pub == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "peer not found"})
		return
	}

	key := store.WgPeerTrafficKey(id, name)
	resp := wgusertraffic.DefaultStore().Query(key, period)
	resp.Peer = name

	iface := strings.TrimSpace(inst.Interface)
	if iface == "" {
		iface = "wg0"
	}
	if inst.Enabled {
		if stats, err := wg.DumpPeerStats(iface); err == nil {
			if row, ok := stats[pub]; ok {
				now := time.Now()
				resp.Online = wg.PeerLikelyOnline(row.LastHandshake, now, wgPeerOnlineHandshakeWindow)
				resp.Current = map[string]any{
					"public_key":          pub,
					"RX":                  row.RxBytes,
					"TX":                  row.TxBytes,
					"rx":                  row.RxBytes,
					"tx":                  row.TxBytes,
					"raw_rx":              row.RxBytes,
					"raw_tx":              row.TxBytes,
					"last_handshake_unix": row.LastHandshake.Unix(),
					"last_handshake_nsec": int64(row.LastHandshake.Nanosecond()),
				}
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func validateWireGuardInstancesUnique(list []store.WireGuardInstance) string {
	seenIface := map[string]string{}
	seenPort := map[int]string{}
	for _, inst := range list {
		iid := strings.TrimSpace(inst.ID)
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if other, ok := seenIface[iface]; ok && other != iid {
			return fmt.Sprintf("duplicate interface %s", iface)
		}
		seenIface[iface] = iid
		if inst.Mode == store.WGModeServer {
			p := inst.ListenPort
			if p <= 0 {
				p = 51820
			}
			if other, ok := seenPort[p]; ok && other != iid {
				return fmt.Sprintf("duplicate listen_port %d", p)
			}
			seenPort[p] = iid
		}
	}
	return ""
}
