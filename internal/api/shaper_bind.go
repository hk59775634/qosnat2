package api

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) shaperDefaultDevice(st store.State) string {
	if d := strings.TrimSpace(st.Shaper.Device); d != "" {
		return d
	}
	return srv.env.DevLAN
}

func (srv *Server) profileDevice(p store.ProfileEntry, st store.State) string {
	if d := strings.TrimSpace(p.Device); d != "" {
		return d
	}
	return srv.shaperDefaultDevice(st)
}

func (srv *Server) normalizeProfileDevice(device string) (string, error) {
	dev := strings.TrimSpace(device)
	if dev == "" {
		dev = srv.shaperDefaultDevice(srv.store.Get())
	}
	if !route.LinkExists(dev) {
		return "", errDeviceNotFound(dev)
	}
	return dev, nil
}

func (srv *Server) ensureShaperDevice(dev string) {
	st := srv.store.Get()
	leaf := st.Shaper.Leaf
	if dev == "" {
		dev = srv.shaperDefaultDevice(st)
	}
	if err := shaper.EnsureDevice(dev, leaf); err != nil {
		return
	}
	// DEV_LAN 由 applyEBPF→AttachTC（egress BPF + ifb ingress）与 ApplyIFBMirred 处理，勿再 AttachTCDevice 覆盖 ingress
	if srv.bpf != nil && srv.bpf.Ready() && dev != srv.env.DevLAN {
		_ = srv.bpf.AttachTCDevice(dev)
	}
}

func (srv *Server) syncShaperDevices() {
	st := srv.store.Get()
	seen := map[string]struct{}{
		srv.shaperDefaultDevice(st): {},
	}
	for _, p := range st.Shaper.Profiles {
		seen[srv.profileDevice(p, st)] = struct{}{}
	}
	if st.VPN.WireGuard.Enabled {
		iface := st.VPN.WireGuard.Interface
		if iface == "" {
			iface = "wg0"
		}
		seen[iface] = struct{}{}
	}
	for dev := range seen {
		srv.ensureShaperDevice(dev)
	}
}

func (srv *Server) shaperProfilesPayload(list []ProfileListItem) map[string]any {
	st := srv.store.Get()
	ifaces, _ := dnsmasq.ListInterfaces()
	if ifaces == nil {
		ifaces = []dnsmasq.Iface{}
	}
	attached := []string{}
	if srv.bpf != nil && srv.bpf.Ready() {
		if st := srv.bpf.Status(); st["attached_devs"] != nil {
			if arr, ok := st["attached_devs"].([]string); ok {
				attached = arr
			}
		}
	}
	if list == nil {
		list = []ProfileListItem{}
	}
	return map[string]any{
		"profiles":          list,
		"bind_device":       srv.shaperDefaultDevice(st),
		"default_device":    srv.shaperDefaultDevice(st),
		"dev_lan":           srv.env.DevLAN,
		"dev_wan":           srv.env.DevWAN,
		"leaf":              st.Shaper.Leaf,
		"fq_flows":          st.Shaper.FQFlows,
		"fq_quantum":        st.Shaper.FQQuantum,
		"interfaces":        ifaces,
		"attached_devices":  attached,
	}
}

func (srv *Server) replayProfileHosts() {
	if srv.hosts == nil {
		return
	}
	st := srv.store.Get()
	for _, p := range store.SortProfilesByID(st.Shaper.Profiles) {
		ip, ok := store.ProfileHostIP(p.CIDR)
		if !ok {
			continue
		}
		rv, err := srv.rateVal(p.Down, p.Up)
		if err != nil {
			continue
		}
		dev := srv.profileDevice(p, st)
		srv.ensureShaperDevice(dev)
		_ = srv.hosts.EnsureHostOnDevice(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor, dev)
	}
}
