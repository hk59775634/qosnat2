package api

import (
	"log"

	"github.com/hk59775634/qosnat2/internal/ebpf"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

// teardownProfileShaper 删除单条 profile 对应的 TC/BPF/HTB 残留（在 state 仍含该条时调用）
func (srv *Server) teardownProfileShaper(cidr string) {
	if cidr == "" {
		return
	}
	st := srv.store.Get()
	p, found := store.ProfileByCIDR(st.Shaper.Profiles, cidr)
	dev := srv.shaperDefaultDevice(st)
	if found && p.Device != "" {
		dev = srv.profileDevice(p, st)
	}
	if srv.env.DevLAN != "" {
		ebpf.ClearIFBMirred(srv.env.DevLAN, []string{cidr})
	}
	srv.purgeHostsInCIDR(cidr, dev)
	if found {
		srv.removeProfileHTBEntry(p, dev)
	} else {
		srv.removeProfileHTB(cidr)
	}
}

func (srv *Server) removeProfileHTBEntry(p store.ProfileEntry, dev string) {
	if srv.hosts == nil {
		return
	}
	if ip, ok := store.ProfileHostIP(p.CIDR); ok {
		_ = srv.hosts.DeleteHostOnDevice(ip, dev)
		if srv.bpf != nil && srv.bpf.Ready() {
			_ = srv.bpf.DeleteHost(ip)
		}
		return
	}
	srv.hosts.RemoveProfileSubnetFromIFB(p.CIDR, p.ID)
}

// purgeHostsInCIDR 清理网段内已建过的 per-IP HTB / active_host
func (srv *Server) purgeHostsInCIDR(cidr, dev string) {
	if srv.hosts == nil || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	purgeIP := func(ip string) {
		if !store.IPInCIDR(ip, cidr) {
			return
		}
		_ = srv.hosts.DeleteHostOnDevice(ip, dev)
		_ = srv.bpf.PurgeActive(ip)
	}
	if list, err := srv.bpf.ListActive(); err == nil {
		for _, e := range list {
			purgeIP(e.IP)
		}
	}
	_ = srv.bpf.EachClassid(func(ip string, _ uint32) error {
		purgeIP(ip)
		return nil
	})
}

// purgeOrphanShaperHosts 删除已无 BPF 速率匹配的动态 HTB
func (srv *Server) purgeOrphanShaperHosts() {
	if srv.hosts == nil || srv.bpf == nil || !srv.bpf.Ready() {
		return
	}
	seen := map[string]struct{}{}
	try := func(ip string) {
		if ip == "" {
			return
		}
		if _, ok := seen[ip]; ok {
			return
		}
		seen[ip] = struct{}{}
		if _, _, ok := srv.bpf.LookupRates(ip); ok {
			return
		}
		_ = srv.hosts.DeleteHost(ip)
		_ = srv.bpf.PurgeActive(ip)
	}
	if list, err := srv.bpf.ListActive(); err == nil {
		for _, e := range list {
			try(e.IP)
		}
	}
	_ = srv.bpf.EachClassid(func(ip string, _ uint32) error {
		try(ip)
		return nil
	})
}

// rebuildShaperDataPlane 增删 profile 后重建 HTB 根 + BPF map + mirred/u32（与 UI「重建根 qdisc」一致）
func (srv *Server) rebuildShaperDataPlane() {
	if !srv.shaperEnabled() || srv.bpf == nil || !srv.bpf.Ready() || srv.env.DevLAN == "" {
		return
	}
	st := srv.store.Get()
	if srv.hosts != nil {
		srv.hosts.ResetKnown()
	}
	if err := shaper.SetupP0(shaper.Config{
		DevLAN:     srv.env.DevLAN,
		Leaf:       st.Shaper.Leaf,
		FQFlows:    st.Shaper.FQFlows,
		FQQuantum:  st.Shaper.FQQuantum,
		TxQueueLen: st.System.TxQueueLenLAN,
	}); err != nil {
		log.Printf("rebuild SetupP0: %v", err)
		return
	}
	if err := srv.replayAllProfileBPFMaps(st); err != nil {
		log.Printf("rebuild bpf maps: %v", err)
	}
	srv.purgeLegacyHostExact(st)
	srv.syncShaperDevices()
	srv.reattachShaperDataPath()
	srv.purgeOrphanShaperHosts()
	if len(st.Shaper.Profiles) == 0 && st.Shaper.PolicyCIDR == "" {
		_ = shaper.PurgeIFBShaperArtifacts()
		_ = shaper.PurgeLANShaperArtifacts(srv.env.DevLAN)
	}
}

// replayAllProfileBPFMaps 按 state 刷新 profile_lpm 与 /32 host_exact
func (srv *Server) replayAllProfileBPFMaps(st store.State) error {
	if err := srv.bpf.ReplayState(st); err != nil {
		return err
	}
	for _, p := range store.SortProfilesByID(st.Shaper.Profiles) {
		rv, err := srv.rateVal(p.Down, p.Up)
		if err != nil {
			continue
		}
		if err := srv.syncProfileBPFMaps(p.CIDR, rv); err != nil {
			log.Printf("replay bpf profile %s: %v", p.CIDR, err)
		}
	}
	return nil
}

// refreshShaperAfterChange profile 增删后全量重建数据面
func (srv *Server) refreshShaperAfterChange() {
	srv.rebuildShaperDataPlane()
}
