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
		cidr := store.Host32ProfileCIDR(ip)
		if !peerRateEnabled(p) {
			srv.clearPeerProfileShaper(cidr, ip)
			continue
		}
		down, up := peerRateStrings(p)
		rv, err := srv.rateVal(down, up)
		if err != nil {
			log.Printf("wg peer rate %s: %v", ip, err)
			continue
		}
		if err := srv.bpf.UpdateProfile(cidr, rv); err != nil {
			log.Printf("wg bpf profile %s: %v", cidr, err)
			continue
		}
		iface := st.VPN.WireGuard.Interface
		if iface == "" {
			iface = "wg0"
		}
		_ = srv.hosts.EnsureHostOnDevice(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor, iface)
		_ = srv.store.Update(func(s *store.State) {
			srv.assignProfileOnAdd(s, cidr, down, up, 32, iface)
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

func (srv *Server) clearPeerProfileShaper(cidr, ip string) {
	srv.teardownProfileShaper(cidr)
	_ = srv.store.Update(func(s *store.State) {
		var out []store.ProfileEntry
		for _, e := range s.Shaper.Profiles {
			if e.CIDR != cidr {
				out = append(out, e)
			}
		}
		s.Shaper.Profiles = out
	})
	_ = srv.store.Save()
	if srv.bpf != nil && srv.bpf.Ready() {
		_ = srv.bpf.ReplayState(srv.store.Get())
	}
	srv.refreshShaperAfterChange()
}

func (srv *Server) removeWGPeerShaper(peer store.WGPeer) {
	ip := wg.PeerTunnelIP(peer)
	if ip != "" {
		srv.clearPeerProfileShaper(store.Host32ProfileCIDR(ip), ip)
	}
}
