package api

import (
	"log"
	"os/exec"
	"strings"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

func (srv *Server) applyWGShapers(st store.State) {
	if !srv.shaperEnabled() || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	for _, inst := range st.VPN.WireGuards {
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if !inst.Enabled {
			_ = srv.bpf.DetachTCDevice(iface)
			continue
		}
		if err := shaper.SetupEDTDevice(iface, st.Shaper.FQFlows, st.Shaper.FQQuantum); err != nil {
			log.Printf("edt wg device %s: %v", iface, err)
			continue
		}
		if err := srv.bpf.AttachTCDevice(iface); err != nil {
			log.Printf("edt wg attach %s: %v", iface, err)
		}
		srv.syncWGPeerRates(st, inst)
	}
}

func (srv *Server) syncAllWGPeerRates() {
	st := srv.store.Get()
	for _, inst := range st.VPN.WireGuards {
		srv.syncWGPeerRates(st, inst)
	}
}

func (srv *Server) syncWGPeerRates(st store.State, inst store.WireGuardInstance) {
	if !inst.Enabled || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	for _, p := range inst.Peers {
		ip := wg.PeerRateShapeIP(inst, p)
		if ip == "" {
			continue
		}
		if !peerRateEnabled(p) {
			_ = srv.bpf.DeleteHost(ip)
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
		}
	}
}

func (srv *Server) setupOCServShaper(st store.State) {
	prefix := strings.TrimSpace(st.VPN.OCServ.Device)
	if prefix == "" {
		prefix = "vpns"
	}
	devs := listTunDevices(prefix)
	if srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	if !st.VPN.OCServ.Enabled {
		for _, dev := range devs {
			_ = srv.bpf.DetachTCDevice(dev)
			shaper.TeardownDevice(dev)
		}
		return
	}
	for _, dev := range devs {
		if err := shaper.SetupEDTDevice(dev, st.Shaper.FQFlows, st.Shaper.FQQuantum); err != nil {
			log.Printf("edt ocserv %s: %v", dev, err)
			continue
		}
		if err := srv.bpf.AttachTCDevice(dev); err != nil {
			log.Printf("edt ocserv attach %s: %v", dev, err)
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

// removeLegacyIFBPath 清除 HTB/IFB 遗留；须在 EDT BPF attach 之前调用。
func (srv *Server) removeLegacyIFBPath(st store.State) {
	for _, dev := range srv.shaperAllManagedDevices(st) {
		for i := 0; i < 32; i++ {
			out, _ := exec.Command("tc", "filter", "del", "dev", dev, "ingress").CombinedOutput()
			msg := string(out)
			if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") ||
				strings.Contains(msg, "does not match") {
				break
			}
		}
	}
	shaper.TeardownDevice(netif.IFBDev)
	netif.RemoveIFB()
}
