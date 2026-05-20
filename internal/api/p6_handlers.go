package api

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/hk59775634/qosnat2/internal/capture"
	"github.com/hk59775634/qosnat2/internal/conntrack"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

func (srv *Server) captures() *capture.Manager {
	if srv.pcap == nil {
		srv.pcap = capture.New("")
	}
	return srv.pcap
}

func (srv *Server) handleWireGuard(w http.ResponseWriter, r *http.Request) {
	st := srv.store.Get()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{
			"config": st.VPN.WireGuard,
			"status": wg.ShowStatus(st.VPN.WireGuard.Interface),
		})
	case http.MethodPut:
		var body store.WireGuardState
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		_ = srv.store.Update(func(s *store.State) {
			s.VPN.WireGuard = body
		})
		_ = srv.store.Save()
		srv.setupWGShaper()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWireGuardKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	kp, err := wg.GenKeyPair()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	_ = srv.store.Update(func(s *store.State) {
		s.VPN.WireGuard.PrivateKey = kp.Private
		s.VPN.WireGuard.PublicKey = kp.Public
	})
	_ = srv.store.Save()
	writeJSON(w, http.StatusOK, kp)
}

func (srv *Server) handleWireGuardApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	up := st.VPN.WireGuard.Enabled
	if err := wg.Apply(st.VPN.WireGuard, up); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	srv.setupWGShaper()
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "up": up})
}

func (srv *Server) handleWireGuardPeers(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/vpn/wireguard/peers/")
	if strings.HasSuffix(path, "/conf") {
		name := strings.TrimSuffix(path, "/conf")
		if name != "" {
			srv.handleWireGuardPeerConf(w, r, name)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.store.Get().VPN.WireGuard.Peers)
	case http.MethodPost:
		var p store.WGPeer
		if err := readJSON(r, &p); err != nil || p.Name == "" {
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
		_ = srv.store.Update(func(s *store.State) {
			var peers []store.WGPeer
			replaced := false
			for _, e := range s.VPN.WireGuard.Peers {
				if e.Name == p.Name {
					peers = append(peers, p)
					replaced = true
				} else {
					peers = append(peers, e)
				}
			}
			if !replaced {
				peers = append(peers, p)
			}
			s.VPN.WireGuard.Peers = peers
		})
		_ = srv.store.Save()
		srv.syncWGPeerRates()
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		name := r.URL.Query().Get("name")
		if name == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name required"})
			return
		}
		var removed *store.WGPeer
		_ = srv.store.Update(func(s *store.State) {
			var out []store.WGPeer
			for _, e := range s.VPN.WireGuard.Peers {
				if e.Name == name {
					cp := e
					removed = &cp
					continue
				}
				out = append(out, e)
			}
			s.VPN.WireGuard.Peers = out
		})
		_ = srv.store.Save()
		if removed != nil {
			srv.removeWGPeerShaper(*removed)
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleWireGuardPeerConf(w http.ResponseWriter, r *http.Request, name string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	st := srv.store.Get()
	var peer *store.WGPeer
	for i := range st.VPN.WireGuard.Peers {
		if st.VPN.WireGuard.Peers[i].Name == name {
			peer = &st.VPN.WireGuard.Peers[i]
			break
		}
	}
	if peer == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "peer not found"})
		return
	}
	ep := st.VPN.WireGuard.ServerEndpoint
	if ep == "" && srv.env.DevWAN != "" {
		ep = fmt.Sprintf("<WAN_IP>:%d", st.VPN.WireGuard.ListenPort)
	}
	conf := wg.ClientConf(st.VPN.WireGuard, *peer, ep)
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+name+".conf")
	w.Write([]byte(conf))
}

func (srv *Server) handleCaptures(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/v1/diagnostics/captures/") {
		id := strings.TrimPrefix(r.URL.Path, "/api/v1/diagnostics/captures/")
		if strings.HasSuffix(id, "/download") {
			id = strings.TrimSuffix(id, "/download")
			srv.handleCaptureDownload(w, r, id)
			return
		}
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, srv.captures().List())
	case http.MethodPost:
		var body struct {
			Device     string `json:"device"`
			Filter     string `json:"filter"`
			DurationSec int   `json:"duration_sec"`
		}
		if err := readJSON(r, &body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad json"})
			return
		}
		dev := body.Device
		if dev == "" {
			dev = srv.env.DevLAN
		}
		s, err := srv.captures().Start(dev, body.Filter, body.DurationSec)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, s)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id required"})
			return
		}
		_ = srv.captures().Stop(id)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (srv *Server) handleCaptureDownload(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	s, err := srv.captures().Get(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	if _, err := os.Stat(s.File); err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.tcpdump.pcap")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(s.File))
	http.ServeFile(w, r, s.File)
}

func (srv *Server) handleConntrack(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	limit := 200
	if s := r.URL.Query().Get("limit"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			limit = n
		}
	}
	filter := r.URL.Query().Get("filter")
	res, err := conntrack.List(limit, filter)
	if err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, res)
}
