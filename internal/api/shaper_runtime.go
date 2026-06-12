package api

import (
	"log"
	"strings"

	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) shaperEnabled() bool {
	return srv.store.Get().Shaper.Enabled
}

func (srv *Server) shaperRuntimeDevices(st store.State) []string {
	seen := map[string]struct{}{}
	add := func(d string) {
		d = strings.TrimSpace(d)
		if d == "" || !route.LinkExists(d) {
			return
		}
		seen[d] = struct{}{}
	}
	add(srv.env.DevLAN)
	add(srv.shaperDefaultDevice(st))
	for _, p := range st.Shaper.Profiles {
		add(srv.profileDevice(p, st))
	}
	for _, inst := range st.VPN.WireGuards {
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		add(iface)
	}
	out := make([]string, 0, len(seen))
	for d := range seen {
		out = append(out, d)
	}
	return out
}

func (srv *Server) teardownShaperRuntime() {
	st := srv.store.Get()
	devLAN := srv.env.DevLAN
	srv.removeLegacyIFBPath(st)
	for _, dev := range srv.shaperRuntimeDevices(st) {
		if srv.bpf != nil {
			_ = srv.bpf.DetachTCDevice(dev)
		}
		shaper.TeardownDevice(dev)
	}
	if srv.bpf != nil && srv.bpf.Ready() {
		_ = srv.bpf.DetachTC()
		_ = srv.bpf.FlushRuntimeMaps()
	}
	shaper.TeardownEDT(devLAN)
}

func (srv *Server) applyShaperRuntime() {
	st := srv.store.Get()
	srv.applyShaperP0(st)
	srv.applyEBPF(st)
}

func (srv *Server) applyShaperP0(st store.State) {
	if srv.env.DevLAN == "" {
		return
	}
	if err := shaper.SetupEDT(shaper.EDTConfig{
		DevLAN:     srv.env.DevLAN,
		FQFlows:    st.Shaper.FQFlows,
		FQQuantum:  st.Shaper.FQQuantum,
		TxQueueLen: st.System.TxQueueLenLAN,
	}); err != nil {
		log.Printf("edt shaper setup (non-fatal): %v", err)
	}
}

func (srv *Server) applyEBPF(st store.State) {
	if srv.bpf == nil {
		return
	}
	srv.removeLegacyIFBPath(st)
	if !srv.bpf.Ready() {
		if err := srv.bpf.Load(); err != nil {
			log.Printf("edt ebpf load: %v", err)
			return
		}
		log.Printf("edt ebpf loaded")
	}
	if err := srv.bpf.ReplayState(st); err != nil {
		log.Printf("edt ebpf replay: %v", err)
	}
	srv.purgeLegacyHostExact(st)
	srv.syncShaperDevices(st)
	srv.applyWGShapers(st)
	srv.setupOCServShaper(st)
}

func (srv *Server) syncShaperDevices(st store.State) {
	for _, dev := range srv.shaperRuntimeDevices(st) {
		if err := shaper.SetupEDTDevice(dev, st.Shaper.FQFlows, st.Shaper.FQQuantum); err != nil {
			log.Printf("edt device %s: %v", dev, err)
			continue
		}
		if srv.bpf != nil && srv.bpf.Ready() {
			if err := srv.bpf.AttachTCDevice(dev); err != nil {
				log.Printf("edt attach %s: %v", dev, err)
			}
		}
	}
}
