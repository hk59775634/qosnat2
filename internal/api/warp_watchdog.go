package api

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

const (
	warpWatchdogInterval      = 20 * time.Second
	warpWatchdogFailThreshold = 5 // ~100s of consecutive probe failures before reconnect
)

// warpWatchdogAction 看门狗在一次 tick 中采取的动作（测试用决策表）。
type warpWatchdogAction int

const (
	warpWatchNone warpWatchdogAction = iota
	warpWatchHealthy
	warpWatchDebounce
	warpWatchSoftReconnect // veth 修复 + warp-cli connect（不 scrub）
	warpWatchFullReconnect // 栈已不存在时才允许全量 Connect
)

func (a warpWatchdogAction) String() string {
	switch a {
	case warpWatchHealthy:
		return "healthy"
	case warpWatchDebounce:
		return "debounce"
	case warpWatchSoftReconnect:
		return "soft_reconnect"
	case warpWatchFullReconnect:
		return "full_reconnect"
	default:
		return "none"
	}
}

type warpWatchdogSnapshot struct {
	Connected      bool
	OpActive       bool
	NetnsExists    bool
	ServiceRunning bool
}

// evaluateWarpWatchdog 纯决策：连续失败达到阈值才重连；永不自动 ResetBroken。
func evaluateWarpWatchdog(prevFail int, snap warpWatchdogSnapshot) (failCount int, action warpWatchdogAction) {
	if snap.Connected {
		return 0, warpWatchHealthy
	}
	if snap.OpActive {
		// 连接/断开进行中：不计失败、不干预，避免与手动操作抢锁拆栈。
		return prevFail, warpWatchNone
	}
	failCount = prevFail + 1
	if failCount < warpWatchdogFailThreshold {
		return failCount, warpWatchDebounce
	}
	// 有 netns（无论 warp-svc 是否短暂消失）优先软重连：修 veth + 必要时拉起 svc + warp-cli connect。
	if snap.NetnsExists {
		return failCount, warpWatchSoftReconnect
	}
	return failCount, warpWatchFullReconnect
}

var (
	warpWatchFailMu    sync.Mutex
	warpWatchFailCount int
)

func resetWarpWatchdogFailCount() {
	warpWatchFailMu.Lock()
	warpWatchFailCount = 0
	warpWatchFailMu.Unlock()
}

func noteWarpWatchdogFail(n int) {
	warpWatchFailMu.Lock()
	warpWatchFailCount = n
	warpWatchFailMu.Unlock()
}

func currentWarpWatchdogFailCount() int {
	warpWatchFailMu.Lock()
	defer warpWatchFailMu.Unlock()
	return warpWatchFailCount
}

func (srv *Server) startWarpWatchdog() {
	if srv.warpWatchCancel != nil {
		return
	}
	clearStaleWarpTaskOnBoot()
	resetWarpWatchdogFailCount()
	ctx, cancel := context.WithCancel(context.Background())
	srv.warpWatchCancel = cancel
	go func() {
		ticker := time.NewTicker(warpWatchdogInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.warpWatchdogTick()
			}
		}
	}()
}

func (srv *Server) warpWatchdogTick() {
	if !srv.store.Get().Network.WarpEnabled {
		resetWarpWatchdogFailCount()
		return
	}
	if !commandExists("warp-cli") {
		return
	}

	// 轻量保活：仅确保宿主机 NAT，不因 NeedsReset 做破坏性 scrub。
	warpnetns.EnsureHostNATOnly()

	snap := warpWatchdogSnapshot{
		Connected:      warpnetns.IsConnected(),
		OpActive:       warpnetns.OpActive(),
		NetnsExists:    warpnetns.NetnsExists(),
		ServiceRunning: warpnetns.ServiceRunning(),
	}
	prev := currentWarpWatchdogFailCount()
	failCount, action := evaluateWarpWatchdog(prev, snap)
	noteWarpWatchdogFail(failCount)

	switch action {
	case warpWatchHealthy:
		warpnetns.RefreshConnectedState()
		srv.syncWarpStoreWhenEnabled()
		return
	case warpWatchNone:
		return
	case warpWatchDebounce:
		if failCount == 1 || failCount == warpWatchdogFailThreshold-1 {
			log.Printf("warp watchdog: probe down (%d/%d), waiting before reconnect",
				failCount, warpWatchdogFailThreshold)
		}
		return
	case warpWatchSoftReconnect:
		log.Printf("warp watchdog: soft reconnect after %d consecutive probe failures", failCount)
		if err := warpnetns.TrySoftReconnect(); err != nil {
			log.Printf("warp watchdog soft reconnect: %v", err)
		}
		if warpnetns.IsConnected() {
			resetWarpWatchdogFailCount()
			iface := warpHostIface()
			if err := srv.applyWarpWanLink(iface); err != nil {
				log.Printf("warp watchdog soft apply: %v", err)
			}
			srv.syncWarpStoreWhenEnabled()
			return
		}
		// 软重连仍失败：保持失败计数，下一轮可再试；不立刻全量拆栈。
		return
	case warpWatchFullReconnect:
		log.Printf("warp watchdog: full reconnect after %d failures (netns missing)", failCount)
		srv.ensureWarpTunnelAsync("watchdog")
		return
	}
}

func warpHostIface() string {
	iface := warpnetns.HostInterface()
	if iface == "" {
		iface = "qwp0"
	}
	return iface
}

func (srv *Server) syncWarpStoreWhenEnabled() {
	if warpnetns.OpActive() {
		return
	}
	st := srv.store.Get()
	if !st.Network.WarpEnabled {
		return
	}
	iface := warpHostIface()
	_ = srv.store.Update(func(st *store.State) {
		store.UpsertWarpWanLink(st, iface)
		store.SyncWanRoutes(st)
		store.SyncEgressRoutes(st)
	})
	if err := srv.store.Save(); err != nil {
		log.Printf("warp store sync: %v", err)
	}
}

func (srv *Server) ensureWarpTunnelAsync(reason string) {
	if !srv.store.Get().Network.WarpEnabled {
		return
	}
	if !commandExists("warp-cli") {
		return
	}
	if warpnetns.OpActive() {
		return
	}
	st := getWarpTaskStatus()
	if st.State == warpInstallStateRunning {
		return
	}
	// 刚失败不久时留给用户/UI 一次干净重试，避免与手动「启用」并发拆 netns。
	if st.State == warpInstallStateFailed && st.FinishedAt != "" {
		if t, err := time.Parse(time.RFC3339, st.FinishedAt); err == nil && time.Since(t) < 45*time.Second {
			return
		}
	}
	if err := srv.startWarpConnectAsync(nil); err != nil {
		if reason != "" {
			log.Printf("warp ensure (%s): %v", reason, err)
		}
	}
}
