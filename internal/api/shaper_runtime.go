package api

import (
	"log"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) shaperEnabled() bool {
	return srv.store.Get().Shaper.Enabled
}

func (srv *Server) stopShaperBackground() {
	if srv.ringCancel != nil {
		srv.ringCancel()
		srv.ringCancel = nil
	}
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

// teardownShaperRuntime 卸载运行中的 TC/eBPF 整形，保留 state 中策略配置供再次启用。
func (srv *Server) teardownShaperRuntime() {
	st := srv.store.Get()
	if srv.usesEDTShaper(st) {
		srv.teardownShaperRuntimeEDT(st)
		return
	}
	srv.stopShaperBackground()
	devLAN := srv.env.DevLAN

	if devLAN != "" {
		if err := ebpf.ResetIFBMirred(devLAN, nil); err != nil {
			log.Printf("teardown ifb mirred %s: %v", devLAN, err)
		}
		ebpf.RemoveLANIngressBPF(devLAN)
	}

	if srv.bpf != nil && srv.bpf.Ready() {
		for _, dev := range srv.shaperRuntimeDevices(st) {
			if dev == devLAN {
				continue
			}
			if err := ebpf.ResetIFBMirredOnDevice(dev, nil, false); err != nil {
				log.Printf("teardown mirred %s: %v", dev, err)
			}
			_ = srv.bpf.DetachTCDevice(dev)
			shaper.TeardownDevice(dev)
		}
		if err := srv.bpf.DetachTC(); err != nil {
			log.Printf("teardown bpf detach: %v", err)
		}
		if err := srv.bpf.FlushRuntimeMaps(); err != nil {
			log.Printf("teardown bpf flush maps: %v", err)
		}
	}

	if srv.hosts != nil {
		srv.hosts.ResetKnown()
	}
	shaper.Teardown(devLAN)
}

// applyShaperRuntime 从 state 启用 QoS 数据面（SetupP0 + eBPF + mirred）。
func (srv *Server) applyShaperRuntime() {
	st := srv.store.Get()
	srv.applyShaperP0(st)
	srv.applyEBPF(st)
}
