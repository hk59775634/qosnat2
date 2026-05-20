package api

import (
	"log"
	"strings"

	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

func (srv *Server) setupWGShaper() {
	st := srv.store.Get()
	wgc := st.VPN.WireGuard
	iface := wgc.Interface
	if iface == "" {
		iface = "wg0"
	}
	leaf := st.Shaper.Leaf

	if !wgc.Enabled {
		srv.hosts.SetExtraDev("")
		if srv.bpf != nil {
			_ = srv.bpf.DetachTCDevice(iface)
		}
		return
	}

	if err := shaper.EnsureDevice(iface, leaf); err != nil {
		log.Printf("wg shaper device %s: %v", iface, err)
		return
	}
	srv.hosts.SetExtraDev(iface)
	if srv.bpf != nil && srv.bpf.Ready() {
		if err := srv.bpf.AttachTCDevice(iface); err != nil {
			log.Printf("wg bpf attach %s: %v", iface, err)
		}
	}
	srv.syncWGPeerRates()
}

func (srv *Server) syncWGPeerRates() {
	if srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	st := srv.store.Get()
	for _, p := range st.VPN.WireGuard.Peers {
		ip := wg.PeerTunnelIP(p)
		if ip == "" {
			continue
		}
		if !peerRateEnabled(p) {
			srv.clearPeerShaper(ip)
			continue
		}
		down, up := peerRateStrings(p)
		rv, err := srv.rateVal(down, up)
		if err != nil {
			log.Printf("wg peer rate %s: %v", ip, err)
			continue
		}
		if err := srv.bpf.UpdateHost(ip, rv); err != nil {
			log.Printf("wg bpf host %s: %v", ip, err)
			continue
		}
		if err := srv.hosts.EnsureHost(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor); err != nil {
			log.Printf("wg htb %s: %v", ip, err)
		}
		_ = srv.store.Update(func(s *store.State) {
			s.Shaper.Hosts[ip] = store.HostRate{Down: down, Up: up}
		})
	}
	_ = srv.store.Save()
}

func peerRateEnabled(p store.WGPeer) bool {
	if p.Rate == nil {
		return false
	}
	return strings.TrimSpace(p.Rate.Down) != "" || strings.TrimSpace(p.Rate.Up) != ""
}

func peerRateStrings(p store.WGPeer) (down, up string) {
	down = strings.TrimSpace(p.Rate.Down)
	up = strings.TrimSpace(p.Rate.Up)
	if down == "" {
		down = up
	}
	if up == "" {
		up = down
	}
	return down, up
}

func (srv *Server) clearPeerShaper(ip string) {
	if srv.bpf != nil && srv.bpf.Ready() {
		_ = srv.bpf.DeleteHost(ip)
	}
	_ = srv.hosts.DeleteHost(ip)
	_ = srv.store.Update(func(s *store.State) {
		delete(s.Shaper.Hosts, ip)
	})
	_ = srv.store.Save()
}

func (srv *Server) removeWGPeerShaper(peer store.WGPeer) {
	ip := wg.PeerTunnelIP(peer)
	if ip != "" {
		srv.clearPeerShaper(ip)
	}
}
