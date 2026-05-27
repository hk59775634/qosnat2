package api

import (
	"log"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

func (srv *Server) setupWGShaper() {
	st := srv.store.Get()
	leaf := st.Shaper.Leaf

	enabledCount := 0
	var firstIface string
	for i := range st.VPN.WireGuards {
		inst := st.VPN.WireGuards[i]
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if !inst.Enabled {
			if route.LinkExists(iface) {
				if err := ebpf.ResetIFBMirredOnDevice(iface, nil, false); err != nil {
					log.Printf("wg mirred flush %s: %v", iface, err)
				}
			}
			if srv.bpf != nil {
				_ = srv.bpf.DetachTCDevice(iface)
			}
			continue
		}
		enabledCount++
		if firstIface == "" {
			firstIface = iface
		}
		if err := shaper.EnsureDevice(iface, leaf); err != nil {
			log.Printf("wg shaper device %s: %v", iface, err)
			continue
		}
		if srv.bpf != nil && srv.bpf.Ready() {
			if err := srv.bpf.AttachTCDeviceEgressOnly(iface); err != nil {
				log.Printf("wg bpf attach %s: %v", iface, err)
			}
		}
	}
	if enabledCount == 0 {
		srv.hosts.SetExtraDev("")
		return
	}
	// 兼容 EnsureHost() 路径：仅单实例启用时把 extraDev 指到该 wg 口
	if enabledCount == 1 {
		srv.hosts.SetExtraDev(firstIface)
	} else {
		srv.hosts.SetExtraDev("")
	}
	srv.applyWireGuardMirred()
	srv.syncWGPeerRates()
}

// applyWireGuardMirred 将各已启用实例隧道源网段从对应 wg 口 ingress mirred 到 ifb0
func (srv *Server) applyWireGuardMirred() {
	st := srv.store.Get()
	for i := range st.VPN.WireGuards {
		inst := st.VPN.WireGuards[i]
		if !inst.Enabled {
			continue
		}
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if !route.LinkExists(iface) {
			continue
		}
		cidrs := store.WireGuardMirredSrcCIDRs(inst.WireGuardState)
		if len(cidrs) == 0 {
			continue
		}
		bidir := inst.Mode == store.WGModeClient
		if err := ebpf.ResetIFBMirredOnDevice(iface, cidrs, bidir); err != nil {
			log.Printf("wg ifb mirred %s: %v", iface, err)
		}
	}
}

func (srv *Server) syncWGPeerRates() {
	if srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	st := srv.store.Get()
	for _, inst := range st.VPN.WireGuards {
		if !inst.Enabled {
			continue
		}
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		for _, p := range inst.Peers {
			ip := wg.PeerRateShapeIP(inst, p)
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
			_ = srv.hosts.EnsureHostOnDevice(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor, iface)
			_ = srv.store.Update(func(s *store.State) {
				srv.assignProfileOnAdd(s, cidr, down, up, 32, iface)
			})
		}
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

func (srv *Server) removeWGPeerShaper(inst store.WireGuardInstance, peer store.WGPeer) {
	ip := wg.PeerRateShapeIP(inst, peer)
	if ip != "" {
		srv.clearPeerProfileShaper(store.Host32ProfileCIDR(ip), ip)
	}
}
