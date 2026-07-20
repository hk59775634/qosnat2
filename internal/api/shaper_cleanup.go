package api

import (
	"log"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) teardownProfileShaper(cidr string) {
	if cidr == "" || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	_ = srv.bpf.DeleteProfile(cidr)
	if ip, ok := store.ProfileHostIP(cidr); ok {
		_ = srv.bpf.DeleteHost(ip)
	}
}

func (srv *Server) rebuildShaperDataPlane() {
	if !srv.shaperEnabled() || srv.bpf == nil || !srv.bpf.Ready() || srv.env.DevLAN == "" {
		return
	}
	st := srv.store.Get()
	if err := srv.replayAllProfileBPFMaps(st); err != nil {
		log.Printf("rebuild edt bpf maps: %v", err)
	}
	srv.purgeLegacyHostExact(st)
	srv.syncShaperDevices(st)
	srv.applyWGShapers(st)
	srv.setupOCServShaper(st)
}

func (srv *Server) replayAllProfileBPFMaps(st store.State) error {
	if err := srv.bpf.ReplayState(st); err != nil {
		return err
	}
	for _, p := range store.SortProfilesByID(st.Shaper.Profiles) {
		rv, err := srv.rateVal(p.Down, p.Up, p.Mask)
		if err != nil {
			continue
		}
		if err := srv.syncProfileBPFMaps(p.CIDR, rv); err != nil {
			log.Printf("replay bpf profile %s: %v", p.CIDR, err)
		}
	}
	return nil
}

func (srv *Server) refreshShaperAfterChange() {
	srv.rebuildShaperDataPlane()
}
