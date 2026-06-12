package api

import (
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
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
	if err := netif.ValidateIfaceName(dev); err != nil {
		return "", err
	}
	if !route.LinkExists(dev) {
		return "", errDeviceNotFound(dev)
	}
	return dev, nil
}

func (srv *Server) shapeDeviceForIP(ip string, st store.State) string {
	cidr := store.Host32ProfileCIDR(ip)
	for _, p := range st.Shaper.Profiles {
		if p.CIDR == cidr {
			return srv.profileDevice(p, st)
		}
	}
	for _, inst := range st.VPN.WireGuards {
		if !inst.Enabled {
			continue
		}
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		for _, p := range inst.Peers {
			if wg.PeerRateShapeIP(inst, p) != ip {
				continue
			}
			if p.Rate != nil && (strings.TrimSpace(p.Rate.Down) != "" || strings.TrimSpace(p.Rate.Up) != "") {
				return iface
			}
		}
	}
	return srv.shaperDefaultDevice(st)
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
		"enabled":          st.Shaper.Enabled,
		"mode":             store.ShaperModeEDT,
		"profiles":         list,
		"bind_device":      srv.shaperDefaultDevice(st),
		"default_device":   srv.shaperDefaultDevice(st),
		"dev_lan":          srv.env.DevLAN,
		"dev_wan":          srv.env.DevWAN,
		"leaf":             st.Shaper.Leaf,
		"fq_flows":         st.Shaper.FQFlows,
		"fq_quantum":       st.Shaper.FQQuantum,
		"interfaces":       ifaces,
		"attached_devices": attached,
	}
}
