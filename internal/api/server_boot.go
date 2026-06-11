package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/shaper"
	"github.com/hk59775634/qosnat2/internal/store"
)

// ApplyAll 启动/回放：sysctl → nft(NAT) → tc → 路由（未完成引导时跳过）
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
	cfg := srv.nftCfg()
	if ips, auto := nft.ResolveSharedIPs(cfg, st); len(ips) == 0 {
		log.Printf("warn: shared_ips empty and no IPv4 on WAN %s, nft SNAT uses masquerade only", srv.env.DevWAN)
	} else if auto {
		log.Printf("shared_ips: using WAN %s address %s", srv.env.DevWAN, ips[0])
	}
	if err := srv.applyNatStackLenient(); err != nil {
		return err
	}
	if srv.shaperEnabled() {
		srv.applyShaperP0(st)
	} else {
		srv.teardownShaperRuntime()
	}
	srv.replayWanLinksOnBoot()
	srv.replayEgressOnBoot()
	netplanApplied := srv.applyNetworkVLANs()
	srv.applyManagedRoutesWithRetry()
	if netplanApplied {
		time.Sleep(3 * time.Second)
		srv.applyManagedRoutesWithRetry()
	}
	srv.applyEgressPolicyRoutes()
	if srv.shaperEnabled() {
		srv.applyEBPF(st)
	}
	return nil
}

func (srv *Server) applyShaperP0(st store.State) {
	if srv.env.DevLAN == "" {
		return
	}
	if srv.usesEDTShaper(st) {
		srv.applyShaperP0EDT(st)
		return
	}
	if err := shaper.SetupP0(shaper.Config{
		DevLAN:     srv.env.DevLAN,
		Leaf:       st.Shaper.Leaf,
		FQFlows:    st.Shaper.FQFlows,
		FQQuantum:  st.Shaper.FQQuantum,
		TxQueueLen: st.System.TxQueueLenLAN,
	}); err != nil {
		log.Printf("shaper setup (non-fatal): %v", err)
	}
}

func (srv *Server) applyManagedRoutesWithRetry() {
	const attempts = 3
	for i := 0; i < attempts; i++ {
		if i > 0 {
			time.Sleep(2 * time.Second)
		}
		st := srv.store.Get()
		if _, err := route.ApplyFromState(st); err != nil {
			log.Printf("routes apply (attempt %d/%d): %v", i+1, attempts, err)
			continue
		}
		return
	}
}

func (srv *Server) applyNetworkVLANs() bool {
	applied, err := srv.applyNetplan()
	if err != nil {
		log.Printf("netplan apply: %v", err)
		return applied
	}
	return applied
}

// ApplyAllOnBoot 同步首次 apply；失败或 nft 表缺失时在后台重试；已引导完成则触发 qos-nat oneshot。
func (srv *Server) ApplyAllOnBoot() {
	var bootErr error
	if err := srv.ApplyAll(); err != nil {
		bootErr = err
		log.Printf("apply on start: %v", err)
	} else if !nft.TableExists() {
		bootErr = fmt.Errorf("inet %s table missing after apply", nft.TableName)
		log.Printf("apply on start: %v", bootErr)
	}
	if bootErr != nil {
		go srv.bootApplyRetryLoop()
	}
	if srv.setupComplete() {
		go func() {
			if err := enableDataplaneOneshot(); err != nil {
				log.Printf("qos-nat oneshot: %v", err)
			}
		}()
	}
}

func (srv *Server) bootApplyRetryLoop() {
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	for i, d := range delays {
		time.Sleep(d)
		if err := srv.ApplyAll(); err != nil {
			log.Printf("apply on start retry %d/%d: %v", i+1, len(delays), err)
			continue
		}
		if !nft.TableExists() {
			log.Printf("apply on start retry %d/%d: nft table still missing", i+1, len(delays))
			continue
		}
		log.Printf("apply on start: background retry %d succeeded", i+1)
		return
	}
	log.Printf("apply on start: background retries exhausted")
}

// applyEBPF 在 TC 拓扑就绪后加载/附加 eBPF（引导完成或 apply-state 时调用）
func (srv *Server) applyEBPF(st store.State) {
	if srv.bpf == nil {
		return
	}
	srv.ensureBPFMode(st)
	if srv.usesEDTShaper(st) {
		srv.applyEBPFEDT(st)
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

// StartBackground ringbuf + 空闲 GC（HTB 模式；EDT 无 ringbuf）
func (srv *Server) StartBackground() {
	if !srv.shaperEnabled() || srv.bpf == nil || !srv.bpf.Ready() || srv.ringCancel != nil {
		return
	}
	if srv.usesEDTShaper(srv.store.Get()) {
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
