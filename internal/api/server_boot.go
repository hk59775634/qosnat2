package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

// ApplyAll 启动/回放：sysctl → tc → nft（未完成引导时跳过）
func (srv *Server) ApplyAll() error {
	if !srv.setupComplete() {
		log.Printf("apply skipped: initial setup not complete")
		return nil
	}
	if srv.env.DevWAN == "" {
		return fmt.Errorf("DEV_WAN must be set")
	}
	st := srv.store.Get()
	if err := srv.applySystemTuning(st); err != nil {
		log.Printf("system tuning: %v", err)
	}
	if srv.env.DevLAN != "" {
		if err := shaper.SetupP0(shaper.Config{
			DevLAN:    srv.env.DevLAN,
			Leaf:      st.Shaper.Leaf,
			FQFlows:   st.Shaper.FQFlows,
			FQQuantum: st.Shaper.FQQuantum,
		}); err != nil {
			return fmt.Errorf("shaper: %w", err)
		}
	}
	cfg := srv.nftCfg()
	if ips, auto := nft.ResolveSharedIPs(cfg, st); len(ips) == 0 {
		log.Printf("warn: shared_ips empty and no IPv4 on WAN %s, nft SNAT uses masquerade only", srv.env.DevWAN)
	} else if auto {
		log.Printf("shared_ips: using WAN %s address %s", srv.env.DevWAN, ips[0])
	}
	if err := srv.applyNatStack(); err != nil {
		return err
	}
	srv.replayWanLinksOnBoot()
	srv.replayEgressOnBoot()
	srv.applyNetworkVLANs()
	srv.applyManagedRoutes()
	srv.applyEgressPolicyRoutes()
	srv.applyEBPF(st)
	return nil
}

// applyEBPF 在 TC 拓扑就绪后加载/附加 eBPF（引导完成或 apply-state 时调用）
func (srv *Server) applyEBPF(st store.State) {
	if srv.bpf == nil {
		return
	}
	if !srv.bpf.Ready() {
		if err := srv.bpf.Load(); err != nil {
			log.Printf("ebpf load: %v", err)
			return
		}
		log.Printf("ebpf loaded after TC/ifb0 ready")
	}
	if err := srv.bpf.ReplayState(st); err != nil {
		log.Printf("ebpf replay: %v", err)
	}
	srv.purgeLegacyHostExact(st)
	srv.syncShaperDevices()
	srv.replayProfileHosts()
	srv.reattachShaperDataPath()
	srv.replayProfileSubnets()
	srv.syncActiveHostHTB()
	srv.StartBackground()
	srv.setupWGShaper()
}

func (srv *Server) shaperMirredCIDRs(st store.State) []string {
	return store.CollectMirredCIDRs(st.Shaper)
}

// StartBackground ringbuf + 空闲 GC
func (srv *Server) StartBackground() {
	if srv.bpf == nil || !srv.bpf.Ready() || srv.ringCancel != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv.ringCancel = cancel
	if err := srv.bpf.StartRingbuf(ctx, srv.hosts); err != nil {
		log.Printf("ringbuf: %v", err)
		cancel()
		srv.ringCancel = nil
		return
	}
	gc := &shaper.GCRunner{
		Hosts:   srv.hosts,
		BPF:     srv.bpf,
		Timeout: srv.idleTimeout(),
		KeepVIP: srv.gcKeepProfiles,
	}
	interval := srv.idleTimeout() / 2
	if interval < time.Minute {
		interval = time.Minute
	}
	go shaper.StartLoop(ctx.Done(), interval, gc)
	srv.startACMEBackground()
	srv.startManagedCertsBackground()
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				srv.syncActiveHostHTB()
			}
		}
	}()
}
