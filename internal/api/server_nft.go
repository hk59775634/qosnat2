package api

import (
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/warpnetns"
)

func (srv *Server) persistAutoFirewallRules() {
	srv.syncAutoFirewallRules()
	if err := srv.store.Save(); err != nil {
		log.Printf("save state: %v", err)
	}
}

// syncAutoFirewallRules 同步 WAN 入站与端口转发关联的受管防火墙规则（写入 state，不单独 Save）。
func (srv *Server) syncAutoFirewallRules() {
	st := srv.store.Get()
	vp := nft.VPNFirewallFromState(st)
	wanDevs := store.CollectWanInputDevices(srv.env.DevWAN, srv.env.DevLAN, st)
	_ = srv.store.Update(func(s *store.State) {
		synced, _ := store.SyncAutoFilterRules(s.Firewall.FilterRules, wanDevs, srv.env.AdminPort, store.AutoInputVPN{
			OCServEnabled: vp.OCServEnabled,
			OCServTCP:     vp.OCServTCP,
			OCServUDP:     vp.OCServUDP,
			WGPorts:       vp.WGPorts,
		}, s.Firewall.WanPortForwards, srv.env.DevLAN, nft.HairpinAddrResolver(srv.env.DevLAN, srv.env.DevWAN))
		s.Firewall.FilterRules = synced
	})
}

// tryReloadNft 在 setup 完成后重载 nft；失败时记录日志并返回警告文案（不中断已完成的 VPN 等操作）。
func (srv *Server) tryReloadNft() string {
	if !srv.setupComplete() {
		return ""
	}
	if err := srv.reloadNft(); err != nil {
		log.Printf("reload nft: %v", err)
		return err.Error()
	}
	return ""
}

func (srv *Server) reconcileWarpAfterNft() {
	warpnetns.ReconcileHostNAT()
	if warpnetns.OpActive() {
		return
	}
	if warpnetns.IsConnected() {
		_ = warpnetns.ReconcileAfterWanLink()
	}
}

func (srv *Server) withNftApply(fn func() error) error {
	srv.nftApplyMu.Lock()
	defer srv.nftApplyMu.Unlock()
	return fn()
}

func (srv *Server) reloadNft() error {
	return srv.withNftApply(srv.reloadNftLocked)
}

func (srv *Server) reloadNftLocked() error {
	if warns := srv.refreshURLAliasesLocked(); len(warns) > 0 {
		log.Printf("url alias refresh: %v", warns)
	}
	start := time.Now()
	st := srv.store.Get()
	err := nft.Apply(srv.nftCfg(), st)
	srv.dataplaneMetrics.recordNftReload(time.Since(start), err)
	if err != nil {
		return err
	}
	srv.persistAutoFirewallRules()
	srv.reconcileWarpAfterNft()
	return nil
}

// applyWanLinkDataPlane 多 WAN 变更后同步策略路由与 nft。
func (srv *Server) applyWanLinkDataPlane() error {
	if warns := srv.refreshURLAliasesLocked(); len(warns) > 0 {
		log.Printf("url alias refresh: %v", warns)
	}
	if err := srv.applyEgressPolicyRoutes(); err != nil {
		return err
	}
	return srv.reloadNft()
}

func (srv *Server) nftCfg() nft.Config {
	return nft.Config{
		DevLAN:    srv.env.DevLAN,
		DevWAN:    srv.env.DevWAN,
		AdminPort: srv.env.AdminPort,
		VPN:       nft.VPNFirewallFromState(srv.store.Get()),
	}
}
