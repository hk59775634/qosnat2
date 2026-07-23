package api

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/frr"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/route"
)

// ApplyAll 启动/回放：sysctl → nft → tc → 路由；外部辅助组件（dnsmasq/jool/unbound）由后台 reconcile。
func (srv *Server) ApplyAll() error {
	if err := srv.applyAllCore(); err != nil {
		return err
	}
	srv.applyAuxNatStack()
	return nil
}

func (srv *Server) applyAllCore() error {
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
	if err := srv.applyNatStackCore(); err != nil {
		return err
	}
	if srv.shaperEnabled() {
		srv.applyShaperP0(st)
	} else {
		srv.teardownShaperRuntime()
	}
	srv.replayWanLinksOnBoot()
	srv.replayProxyEgressOnBoot()
	srv.replayEgressOnBoot()
	netplanApplied := srv.applyNetworkVLANs()
	if err := netif.ApplyVirtualIPs(srv.store.Get().Network); err != nil {
		log.Printf("virtual ips apply: %v", err)
	}
	srv.applyManagedRoutesWithRetry()
	if netplanApplied {
		time.Sleep(3 * time.Second)
		srv.applyManagedRoutesWithRetry()
		if err := netif.ApplyVirtualIPs(srv.store.Get().Network); err != nil {
			log.Printf("virtual ips re-apply: %v", err)
		}
	}
	// 与 UI 保存出站策略一致：sync → routes → ip rule → nft，避免启动时 nft 早于 SyncEgress 导致策略空洞。
	if err := srv.applyEgressDataPlane(); err != nil {
		log.Printf("egress dataplane on boot: %v", err)
	}
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
	frrBackend := route.NormalizeBackend(st.System.RouteBackend) == route.BackendFRR && frr.PackageInstalled()
	if frrBackend && !frr.ServiceActive() {
		log.Printf("routes apply: frr not active, deferred to route guard")
		return
	}
	for i, d := range delays {
		if i > 0 {
			time.Sleep(d)
		}
		st = srv.store.Get()
		ready, deferred := route.PartitionByDeviceReady(st.Routes)
		if len(deferred) > 0 && i == 0 {
			log.Printf("routes apply: deferring %d route(s) until device exists (%s)",
				len(deferred), strings.Join(route.DeferredRouteDevices(deferred), ", "))
		}
		res, err := route.ApplyManagedRoutesWithDynamic(ready, st.System.RouteBackend, st.DynamicRouting)
		if err != nil {
			log.Printf("routes apply (attempt %d/%d): %v", i+1, len(delays), err)
			continue
		}
		// FRR ApplyManaged 已完整重写托管项；勿因 FIB metric/未选中路由误判 missing 而反复 no+add 冲掉策略表。
		if frrBackend {
			if len(deferred) > 0 {
				log.Printf("routes apply: ok backend=%s applied=%d deferred=%d", res.Backend, res.Applied, len(deferred))
			} else {
				log.Printf("routes apply: ok backend=%s applied=%d", res.Backend, res.Applied)
			}
			return
		}
		if missing, merr := route.MissingManaged(ready); merr == nil && len(missing) > 0 {
			if len(deferred) > 0 {
				log.Printf("routes apply: partial ok backend=%s applied=%d skipped=%d missing=%d deferred=%d",
					res.Backend, res.Applied, res.Skipped, len(missing), len(deferred))
				return
			}
			log.Printf("routes apply (attempt %d/%d): still missing %d route(s)", i+1, len(delays), len(missing))
			continue
		}
		if len(deferred) > 0 {
			log.Printf("routes apply: ok backend=%s applied=%d skipped=%d deferred=%d",
				res.Backend, res.Applied, res.Skipped, len(deferred))
		} else {
			log.Printf("routes apply: ok backend=%s applied=%d skipped=%d", res.Backend, res.Applied, res.Skipped)
		}
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

// ApplyAllOnBoot 后台回放核心数据面；失败或 nft 表缺失时在后台重试；辅助组件由 aux reconcile 负责。
func (srv *Server) ApplyAllOnBoot() {
	var bootErr error
	if err := srv.applyAllCore(); err != nil {
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
		if err := srv.applyAllCore(); err != nil {
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
		srv.startAuxServicesBackground(ctx)
		srv.startWanHealthBackground(ctx)
		srv.startScheduleBackground(ctx)
	})
}
