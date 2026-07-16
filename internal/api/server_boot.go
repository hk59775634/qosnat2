package api

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/frr"
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
	if err := srv.applyLVSFromStore(); err != nil {
		log.Printf("lvs apply: %v", err)
	}
	return nil
}

func (srv *Server) applyManagedRoutesWithRetry() {
	delays := []time.Duration{0, 2 * time.Second, 3 * time.Second, 5 * time.Second, 8 * time.Second, 12 * time.Second}
	st := srv.store.Get()
	if route.NormalizeBackend(st.System.RouteBackend) == route.BackendFRR && frr.PackageInstalled() {
		if !frr.WaitActive(30 * time.Second) {
			log.Printf("routes apply: frr not active after 30s")
		}
	}
	for i, d := range delays {
		if i > 0 {
			time.Sleep(d)
		}
		st = srv.store.Get()
		res, err := route.ApplyFromState(st)
		if err != nil {
			log.Printf("routes apply (attempt %d/%d): %v", i+1, len(delays), err)
			continue
		}
		if missing, merr := route.MissingManaged(st.Routes); merr == nil && len(missing) > 0 {
			log.Printf("routes apply (attempt %d/%d): still missing %d route(s)", i+1, len(delays), len(missing))
			continue
		}
		log.Printf("routes apply: ok backend=%s applied=%d skipped=%d", res.Backend, res.Applied, res.Skipped)
		return
	}
	log.Printf("routes apply: retries exhausted")
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
		ctx, cancel := context.WithCancel(context.Background())
		srv.bootApplyCancel = cancel
		go srv.bootApplyRetryLoop(ctx)
	}
	if srv.setupComplete() {
		go func() {
			if err := enableDataplaneOneshot(); err != nil {
				log.Printf("qos-nat oneshot: %v", err)
			}
		}()
	}
}

func (srv *Server) bootApplyRetryLoop(ctx context.Context) {
	delays := []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}
	for i, d := range delays {
		select {
		case <-ctx.Done():
			return
		case <-time.After(d):
		}
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
		srv.ensureGatewayAptLockdown()
		ctx, cancel := context.WithCancel(context.Background())
		srv.serviceBgCancel = cancel
		srv.startACMEBackground(ctx)
		srv.startManagedCertsBackground(ctx)
		srv.startAliasRefreshBackground(ctx)
		srv.startRouteGuardBackground(ctx)
	})
}
