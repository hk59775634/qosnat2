package api

import (
	"log"
	"path/filepath"
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
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
		st := srv.store.Get()
		if srv.isWireGuardIface(st, dev) {
			_ = srv.bpf.AttachTCDeviceEgressOnly(dev)
		} else {
			_ = srv.bpf.AttachTCDevice(dev)
		}
	}
}

func (srv *Server) isWireGuardIface(st store.State, dev string) bool {
	dev = strings.TrimSpace(dev)
	if dev == "" {
		return false
	}
	for _, inst := range st.VPN.WireGuards {
		if !inst.Enabled {
			continue
		}
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		if dev == iface {
			return true
		}
	}
	return false
}

func (srv *Server) wireGuardIfaceName(st store.State) string {
	for _, inst := range st.VPN.WireGuards {
		if !inst.Enabled {
			continue
		}
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		return iface
	}
	return ""
}

func (srv *Server) syncShaperDevices() {
	if !srv.shaperEnabled() {
		return
	}
	st := srv.store.Get()
	seen := map[string]struct{}{
		srv.shaperDefaultDevice(st): {},
	}
	for _, p := range st.Shaper.Profiles {
		seen[srv.profileDevice(p, st)] = struct{}{}
	}
	for _, inst := range st.VPN.WireGuards {
		if !inst.Enabled {
			continue
		}
		iface := strings.TrimSpace(inst.Interface)
		if iface == "" {
			iface = "wg0"
		}
		seen[iface] = struct{}{}
	}
	for dev := range seen {
		srv.ensureShaperDevice(dev)
	}
}

// shapeDeviceForIP 解析 Per-IP 整形应落在哪块网卡（profile.device / WG peer / 默认 LAN）
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

// syncActiveHostHTB 按 BPF 活跃/classid 表在 ifb/LAN 上补建 HTB 类（/24 等网段模板依赖此路径）
const htbSyncBatchSize = 64

func (srv *Server) syncActiveHostHTB() {
	srv.syncActiveHostHTBWithLimit(htbSyncBatchSize)
}

func (srv *Server) syncActiveHostHTBAll() {
	srv.syncActiveHostHTBWithLimit(0)
}

func (srv *Server) syncActiveHostHTBWithLimit(batchLimit int) {
	if !srv.shaperEnabled() || srv.hosts == nil || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	st := srv.store.Get()
	seen := map[string]struct{}{}
	ensured := 0
	tryEnsure := func(ip string, down, up uint64, minor uint32) {
		if batchLimit > 0 && ensured >= batchLimit {
			return
		}
		dev := srv.shapeDeviceForIP(ip, st)
		if srv.hosts.HostConfiguredOnDevice(ip, dev, minor, down, up) {
			return
		}
		if err := srv.hosts.EnsureHostOnDevice(ip, down, up, minor, dev); err != nil {
			log.Printf("sync htb %s@%s: %v", ip, dev, err)
			return
		}
		ensured++
	}
	entries, err := srv.bpf.ListActive()
	if err == nil {
		for _, e := range entries {
			seen[e.IP] = struct{}{}
			down, up, ok := srv.bpf.LookupRates(e.IP)
			if !ok {
				dev := srv.shapeDeviceForIP(e.IP, st)
				if err := srv.hosts.DeleteHostOnDevice(e.IP, dev); err != nil {
					log.Printf("sync htb purge %s: %v", e.IP, err)
				}
				if err := srv.bpf.PurgeActive(e.IP); err != nil {
					log.Printf("sync active purge %s: %v", e.IP, err)
				}
				continue
			}
			tryEnsure(e.IP, down, up, e.ClassMinor)
		}
	}
	_ = srv.bpf.EachClassid(func(ip string, minor uint32) error {
		if _, ok := seen[ip]; ok {
			return nil
		}
		down, up, ok := srv.bpf.LookupRates(ip)
		if !ok {
			return nil
		}
		tryEnsure(ip, down, up, minor)
		return nil
	})
}

// reattachShaperDataPath 在 setupHTBRoot 重建 HTB 后恢复 u32 mirred 与 BPF（parent 1:）
func (srv *Server) reattachShaperDataPath() {
	if !srv.shaperEnabled() || srv.bpf == nil || !srv.bpf.Ready() || srv.env.DevLAN == "" {
		return
	}
	st := srv.store.Get()
	if err := srv.bpf.AttachTC(srv.env.DevLAN); err != nil {
		log.Printf("reattach AttachTC %s: %v", srv.env.DevLAN, err)
		return
	}
	ingressPin := filepath.Join(ebpf.PinDir, "classify_ingress")
	cidrs := srv.shaperMirredCIDRs(st)
	// ens19 ingress 仅 u32+mirred；勿挂 BPF direct-action（会截断 mirred，导致上行不限速）
	ebpf.RemoveLANIngressBPF(srv.env.DevLAN)
	if err := ebpf.ResetIFBMirred(srv.env.DevLAN, cidrs); err != nil {
		log.Printf("reattach ifb mirred %s: %v", srv.env.DevLAN, err)
	}
	if err := ebpf.ApplyIFBIngressBPF(ingressPin); err != nil {
		log.Printf("reattach ifb ingress bpf: %v", err)
	}
	if err := srv.bpf.AttachLANEgressBPF(srv.env.DevLAN); err != nil {
		log.Printf("reattach lan egress %s: %v", srv.env.DevLAN, err)
	}
	srv.replayProfileUploadHTB()
	srv.syncActiveHostHTBAll()
	if err := srv.verifyUploadPath(cidrs); err != nil {
		log.Printf("shaper upload path: %v", err)
	}
	srv.applyWireGuardMirred()
}

// replayProfileUploadHTB 恢复 ifb 上行 u32+HTB（/32 主机优先，网段按前缀最长优先）
func (srv *Server) replayProfileUploadHTB() {
	if srv.hosts == nil {
		return
	}
	shaper.FlushIFBUploadU32()
	st := srv.store.Get()
	for _, p := range store.SortProfilesByPrefixLen(st.Shaper.Profiles) {
		if _, ok := store.ProfileHostIP(p.CIDR); ok {
			continue
		}
		rv, err := srv.rateVal(p.Down, p.Up)
		if err != nil {
			continue
		}
		if err := srv.hosts.EnsureProfileSubnetOnIFB(p.CIDR, p.ID, rv.UpBPS); err != nil {
			log.Printf("replay subnet ifb %s: %v", p.CIDR, err)
		}
	}
	srv.replayProfileHosts()
}

// replayProfileSubnets 兼容旧调用
func (srv *Server) replayProfileSubnets() {
	srv.replayProfileUploadHTB()
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
		if err := srv.hosts.EnsureHostOnDevice(ip, rv.DownBPS, rv.UpBPS, rv.ClassMinor, dev); err != nil {
			log.Printf("replay host htb %s: %v", ip, err)
		}
	}
}
