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
	if srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	for _, p := range inst.Peers {
		ip := wg.PeerRateShapeIP(inst, p)
		if ip == "" {
			continue
		}
		cidr := store.Host32ProfileCIDR(ip)
		if !peerRateEnabled(p) {
			_ = srv.bpf.DeleteProfile(cidr)
			_ = srv.bpf.DeleteHost(ip)
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
		if err := srv.bpf.UpdateHost(ip, rv); err != nil {
			log.Printf("wg bpf host %s: %v", ip, err)
		}
		_ = srv.store.Update(func(s *store.State) {
			srv.assignProfileOnAdd(s, cidr, down, up, 32, strings.TrimSpace(inst.Interface))
		})
	}
	_ = srv.persistStateOrLog("sync wg peer rates")
}

func (srv *Server) setupOCServShaper(st store.State) {
	if !st.VPN.OCServ.Enabled || srv.bpf == nil || !srv.bpf.Ready() {
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
	for _, dev := range srv.shaperRuntimeDevices(st) {
		_ = exec.Command("tc", "filter", "del", "dev", dev, "ingress").Run()
	}
	shaper.TeardownDevice(netif.IFBDev)
	netif.RemoveIFB()
}
