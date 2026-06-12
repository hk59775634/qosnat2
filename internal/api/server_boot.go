package api

import (
	"fmt"
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/route"
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

// StartBackground 启动 ACME/证书等常驻后台任务。
func (srv *Server) StartBackground() {
	srv.startServiceBackground()
}

func (srv *Server) startServiceBackground() {
	srv.serviceBackgroundOnce.Do(func() {
		srv.startACMEBackground()
		srv.startManagedCertsBackground()
	})
}
