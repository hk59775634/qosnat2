package api

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

func (srv *Server) setupWGShaper() {
	st := srv.store.Get()
	if !srv.shaperEnabled() {
		for _, inst := range st.VPN.WireGuards {
			iface := strings.TrimSpace(inst.Interface)
			if iface == "" {
				iface = "wg0"
			}
			if srv.bpf != nil {
				_ = srv.bpf.DetachTCDevice(iface)
			}
			shaper.TeardownDevice(iface)
		}
		return
	}
	srv.applyWGShapers(st)
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
	_ = srv.persistStateOrLog("clear wg peer profile")
	if srv.bpf != nil && srv.bpf.Ready() {
		_ = srv.bpf.DeleteHost(ip)
		_ = srv.bpf.DeleteProfile(cidr)
	}
	srv.refreshShaperAfterChange()
}

func (srv *Server) removeWGPeerShaper(inst store.WireGuardInstance, peer store.WGPeer) {
	ip := wg.PeerRateShapeIP(inst, peer)
	if ip != "" {
		srv.clearPeerProfileShaper(store.Host32ProfileCIDR(ip), ip)
	}
}
