package api

import (
	"log"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) usesEDTShaper(st store.State) bool {
	return st.Shaper.UsesEDT()
}

func (srv *Server) ensureBPFMode(st store.State) {
	if srv.bpf == nil {
		return
	}
	mode := store.EffectiveShaperMode(st.Shaper)
	if srv.bpf.Ready() && srv.bpf.Mode() != mode {
		srv.teardownShaperRuntime()
		_ = srv.bpf.Close()
	}
	srv.bpf.SetMode(mode)
}

func (srv *Server) applyShaperP0EDT(st store.State) {
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

func (srv *Server) applyEBPFEDT(st store.State) {
	if srv.bpf == nil {
		return
	}
	srv.ensureBPFMode(st)
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
	srv.syncShaperDevicesEDT(st)
	srv.setupWGShaperEDT(st)
	srv.setupOCServShaperEDT(st)
}

func (srv *Server) syncShaperDevicesEDT(st store.State) {
	for _, dev := range srv.shaperRuntimeDevices(st) {
		if err := shaper.SetupEDTDevice(dev, st.Shaper.FQFlows, st.Shaper.FQQuantum); err != nil {
			log.Printf("edt device %s: %v", dev, err)
			continue
		}
		if srv.bpf != nil && srv.bpf.Ready() {
			if err := srv.bpf.AttachTCDeviceEDT(dev); err != nil {
				log.Printf("edt attach %s: %v", dev, err)
			}
		}
	}
}

func (srv *Server) setupWGShaperEDT(st store.State) {
	if !srv.shaperEnabled() || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	for _, inst := range st.VPN.WireGuards {
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if !inst.Enabled {
			if route.LinkExists(iface) {
				_ = ebpf.ResetIFBMirredOnDevice(iface, nil, false)
			}
			_ = srv.bpf.DetachTCDevice(iface)
			continue
		}
		if err := shaper.SetupEDTDevice(iface, st.Shaper.FQFlows, st.Shaper.FQQuantum); err != nil {
			log.Printf("edt wg device %s: %v", iface, err)
			continue
		}
		if err := srv.bpf.AttachTCDeviceEDT(iface); err != nil {
			log.Printf("edt wg attach %s: %v", iface, err)
		}
		srv.syncWGPeerRatesEDT(st, inst)
	}
}

func (srv *Server) syncWGPeerRatesEDT(st store.State, inst store.WireGuardInstance) {
	_ = st
	_ = inst
	srv.syncWGPeerRates()
}

func (srv *Server) setupOCServShaperEDT(st store.State) {
	if !st.VPN.OCServ.Enabled {
		return
	}
	prefix := strings.TrimSpace(st.VPN.OCServ.Device)
	if prefix == "" {
		prefix = "vpns"
	}
	for _, dev := range listTunDevices(prefix) {
		if err := shaper.SetupEDTDevice(dev, st.Shaper.FQFlows, st.Shaper.FQQuantum); err != nil {
			log.Printf("edt ocserv %s: %v", dev, err)
			continue
		}
		if srv.bpf != nil && srv.bpf.Ready() {
			if err := srv.bpf.AttachTCDeviceEDT(dev); err != nil {
				log.Printf("edt ocserv attach %s: %v", dev, err)
			}
		}
	}
}

func listTunDevices(prefix string) []string {
	out, err := exec.Command("ip", "-o", "link", "show").CombinedOutput()
	if err != nil {
		return nil
	}
	var devs []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimSuffix(fields[1], ":")
		if strings.HasPrefix(name, prefix) {
			devs = append(devs, name)
		}
	}
	return devs
}

func (srv *Server) teardownShaperRuntimeEDT(st store.State) {
	srv.stopShaperBackground()
	devLAN := srv.env.DevLAN
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
