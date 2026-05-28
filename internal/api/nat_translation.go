package api

import (
	"fmt"
	"log"
	"strings"

	"github.com/hk59775634/qosnat2/internal/dnsmasq"
	"github.com/hk59775634/qosnat2/internal/jool"
	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/nft"
	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/unbound"
)

func (srv *Server) dnsmasqOpts(st store.State) dnsmasq.ApplyOpts {
	opts := dnsmasq.ApplyOpts{
		ExceptWAN: srv.env.DevWAN,
		DevLAN:    srv.env.DevLAN,
		Nat:       st.Nat,
	}
	iface := st.DHCP.Interface
	if iface == "" {
		iface = srv.env.DevLAN
	}
	if v6, err := netif.PrimaryIPv6(iface); err == nil {
		opts.LANIPv6 = v6
	}
	if v4, err := netif.PrimaryIPv4(iface); err == nil {
		opts.LANIPv4 = v4
	}
	return opts
}

// applyNatStack nft + Jool + Unbound + dnsmasq（NAT64/NPTv6/DNS64 变更时）
func (srv *Server) applyNatStack() error {
	st := srv.store.Get()
	if st.Nat.Nat64Enabled || st.Nat.Nptv6Enabled {
		_ = srv.store.Update(func(s *store.State) {
			if s.System.Sysctl == nil {
				s.System.Sysctl = map[string]string{}
			}
			s.System.Sysctl["net.ipv6.conf.all.forwarding"] = "1"
			s.System.Sysctl["net.ipv6.conf.default.forwarding"] = "1"
		})
		_ = srv.store.Save()
		st = srv.store.Get()
		if err := srv.applySystemTuning(st); err != nil {
			log.Printf("ipv6 forwarding sysctl: %v", err)
		}
	}
	if err := nft.Apply(srv.nftCfg(), st); err != nil {
		return fmt.Errorf("nft: %w", err)
	}
	srv.persistAutoFirewallRules()
	if err := jool.Apply(st.Nat); err != nil {
		return fmt.Errorf("jool: %w", err)
	}
	opts := srv.unboundOpts(st)
	if err := unbound.Apply(st.Nat, opts); err != nil {
		return fmt.Errorf("unbound: %w", err)
	}
	if err := srv.applyDNSMasqNAT(st); err != nil {
		return fmt.Errorf("dnsmasq: %w", err)
	}
	return nil
}

func (srv *Server) unboundOpts(st store.State) unbound.RenderOpts {
	iface := st.DHCP.Interface
	if iface == "" {
		iface = srv.env.DevLAN
	}
	gw4, _ := netif.PrimaryIPv4(iface)
	return unbound.RenderOpts{
		GatewayIPv4: gw4,
		AccessAllow: store.CollectDNS64AccessAllow(st),
	}
}

func (srv *Server) applyDNSMasqNAT(st store.State) error {
	if !st.DHCP.Enabled && !st.Nat.DNS64UsesDnsmasqRelay() {
		return nil
	}
	cfg := st.DHCP
	if cfg.Interface == "" {
		cfg.Interface = srv.env.DevLAN
	}
	if !cfg.Enabled && !st.Nat.DNS64UsesDnsmasqRelay() {
		return nil
	}
	if err := store.NormalizeDHCP(&cfg, srv.env.DevLAN); err != nil {
		log.Printf("dhcp normalize: %v", err)
		return nil
	}
	return dnsmasq.Apply(cfg, srv.dnsmasqOpts(st))
}

func (srv *Server) nat64Status(st store.State) map[string]any {
	opts := srv.unboundOpts(st)
	return map[string]any{
		"nat64_enabled":     st.Nat.Nat64Enabled,
		"nat64_prefix":      st.Nat.Nat64Prefix,
		"nat64_pool4":       st.Nat.Nat64Pool4,
		"dns64":             st.Nat.DNS64,
		"dns64_direct":      st.Nat.DNS64DirectToClients(),
		"dns64_dnsmasq":     st.Nat.DNS64UsesDnsmasqRelay(),
		"unbound_listen":    unbound.ListenSummary(st.Nat, opts.GatewayIPv4),
		"recommended_dns":   srv.recommendedDNS64(st),
		"jool_active":       jool.Active(),
		"unbound_active":    unbound.Active(),
		"jool_installed":    jool.Installed(),
		"unbound_installed": unbound.Installed(),
	}
}

func (srv *Server) recommendedDNS64(st store.State) map[string]any {
	if !st.Nat.Nat64Enabled {
		return nil
	}
	if st.Nat.DNS64.Mode == store.DNS64ModeUpstream {
		return map[string]any{
			"mode":    "upstream",
			"servers": st.Nat.EffectiveDNS64Upstream(),
			"hint":    "Configure VPN clients to use these DNS64 resolvers (not normal Google IPv6 DNS).",
		}
	}
	opts := srv.unboundOpts(st)
	host, port, _ := st.Nat.DNS64.EffectiveUnboundListen(opts.GatewayIPv4)
	addr := host
	if strings.Contains(host, ":") {
		addr = "[" + host + "]"
	}
	return map[string]any{
		"mode":     "local_unbound",
		"address":  addr,
		"port":     port,
		"hint":     "Point VPN DNS to this gateway address (no dnsmasq/DHCP required).",
	}
}
