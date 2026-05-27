package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const EgressTableBase = 201

// EgressPolicy 将源网段流量导向指定 WanLink（Linux 策略路由 + 对应 WAN 口 SNAT）
type EgressPolicy struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	CIDR      string `json:"cidr"`
	WanLinkID string `json:"wan_link_id"`
	SNATIP    string `json:"snat_ip,omitempty"` // 空则使用该 WAN 口首个全球 IPv4
	Priority  int    `json:"priority"`          // ip rule 优先级，越小越优先；默认 100
	Enabled   bool   `json:"enabled"`
}

func NewEgressPolicyID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "eg-" + hex.EncodeToString(b[:])
}

// NormalizeEgressPolicy 校验并补全出站策略
func NormalizeEgressPolicy(p *EgressPolicy) error {
	if p == nil {
		return fmt.Errorf("egress policy nil")
	}
	if p.ID == "" {
		p.ID = NewEgressPolicyID()
	}
	p.Name = strings.TrimSpace(p.Name)
	p.CIDR = strings.TrimSpace(p.CIDR)
	p.WanLinkID = strings.TrimSpace(p.WanLinkID)
	p.SNATIP = strings.TrimSpace(p.SNATIP)
	if p.CIDR == "" {
		return fmt.Errorf("cidr required")
	}
	if err := ValidateCIDR(p.CIDR); err != nil {
		return err
	}
	if p.WanLinkID == "" {
		return fmt.Errorf("wan_link_id required")
	}
	if p.SNATIP != "" {
		if err := ValidateIPv4OrCIDR(p.SNATIP); err != nil {
			return fmt.Errorf("snat_ip: %w", err)
		}
		if strings.Contains(p.SNATIP, "/") {
			return fmt.Errorf("snat_ip must be host address, not cidr")
		}
	}
	if p.Priority <= 0 {
		p.Priority = 100
	}
	return nil
}

// FindWanLink 按 ID 查找多 WAN 链路
func FindWanLink(links []WanLink, id string) (WanLink, bool) {
	for _, w := range links {
		if w.ID == id {
			return w, true
		}
	}
	return WanLink{}, false
}

// WanLinkRouteTable 为每条启用的 WanLink 分配独立路由表（201 起，按 ID 排序保持稳定）
func WanLinkRouteTable(wanLinkID string, links []WanLink) int {
	var ids []string
	for _, w := range links {
		if strings.TrimSpace(w.ID) != "" {
			ids = append(ids, w.ID)
		}
	}
	sort.Strings(ids)
	for i, id := range ids {
		if id == wanLinkID {
			return EgressTableBase + i
		}
	}
	return 0
}

// EnabledEgressPolicies 返回已启用的出站策略（按 priority、id 排序）
func EnabledEgressPolicies(policies []EgressPolicy) []EgressPolicy {
	var out []EgressPolicy
	for _, p := range policies {
		if p.Enabled {
			out = append(out, p)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Priority != out[j].Priority {
			return out[i].Priority < out[j].Priority
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// EgressPolicyCIDRs 返回所有已启用出站策略的 CIDR（用于从主 WAN SNAT 中排除）
func EgressPolicyCIDRs(policies []EgressPolicy) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range EnabledEgressPolicies(policies) {
		if _, ok := seen[p.CIDR]; ok {
			continue
		}
		seen[p.CIDR] = struct{}{}
		out = append(out, p.CIDR)
	}
	return out
}

// FilterPolicyRoutesForWAN 从 policy_routes 中去掉已由出站策略接管的 CIDR
func FilterPolicyRoutesForWAN(policyRoutes, egressCIDRs []string) []string {
	if len(egressCIDRs) == 0 {
		return policyRoutes
	}
	egressSet := map[string]struct{}{}
	for _, c := range egressCIDRs {
		egressSet[c] = struct{}{}
	}
	var out []string
	for _, c := range policyRoutes {
		if _, skip := egressSet[c]; skip {
			continue
		}
		out = append(out, c)
	}
	return out
}

// ResolvedEgress 解析后的出站策略（含 WAN 设备、网关、路由表、SNAT 地址）
type ResolvedEgress struct {
	Policy   EgressPolicy `json:"policy"`
	WanLink  WanLink      `json:"wan_link"`
	Device   string       `json:"device"`
	Gateway  string       `json:"gateway"`
	Table    int          `json:"table"`
	SNATIP   string       `json:"snat_ip"`
	Priority int          `json:"priority"`
}

// ResolveEgressPolicies 解析出站策略；跳过无效或缺失 WanLink 的项
func ResolveEgressPolicies(st State, primaryIP func(device string) (string, error)) []ResolvedEgress {
	var out []ResolvedEgress
	for _, p := range EnabledEgressPolicies(st.Network.EgressPolicies) {
		w, ok := FindWanLink(st.Network.WanLinks, p.WanLinkID)
		if !ok || !w.Enabled {
			continue
		}
		dev := strings.TrimSpace(w.Device)
		gw := strings.TrimSpace(w.Gateway)
		if dev == "" || gw == "" {
			continue
		}
		tbl := WanLinkRouteTable(w.ID, st.Network.WanLinks)
		if tbl == 0 {
			continue
		}
		snat := p.SNATIP
		if snat == "" && primaryIP != nil {
			snat, _ = primaryIP(dev)
		}
		if snat == "" {
			continue
		}
		out = append(out, ResolvedEgress{
			Policy:   p,
			WanLink:  w,
			Device:   dev,
			Gateway:  gw,
			Table:    tbl,
			SNATIP:   snat,
			Priority: p.Priority,
		})
	}
	return out
}
