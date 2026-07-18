package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

const EgressTableBase = 201

// GoogleIPv4RangesURL Google 官方 IPv4-only 网段列表。
const GoogleIPv4RangesURL = "https://www.gstatic.com/ipranges/goog_ipv4_only.txt"

// EgressPolicy 将源/目的网段流量导向指定 WanLink（Linux 策略路由；默认再叠加该 WAN 口 SNAT）
type EgressPolicy struct {
	ID        string `json:"id"`
	Name      string `json:"name,omitempty"`
	CIDR      string `json:"cidr,omitempty"`  // 兼容旧版：单 CIDR + match
	Match     string `json:"match,omitempty"` // 兼容旧版：source|destination
	SrcCIDR   string `json:"src_cidr,omitempty"`
	DstCIDR   string `json:"dst_cidr,omitempty"`
	SrcAlias  string `json:"src_alias,omitempty"`
	DstAlias  string `json:"dst_alias,omitempty"`
	SrcIface  string `json:"src_iface,omitempty"` // 入接口 iif；可与 src_cidr/alias 组合
	WanLinkID string `json:"wan_link_id"`
	SNATIP    string `json:"snat_ip,omitempty"` // 空则使用该 WAN 口首个全球 IPv4；NoSNAT 时忽略
	// NoSNAT：仅策略路由到 WanLink 网关（如远端 NAT 服务器），本机不做 SNAT/masquerade。
	NoSNAT   bool `json:"no_snat,omitempty"`
	Priority int  `json:"priority"` // ip rule 优先级，越小越优先；默认 100
	Enabled  bool `json:"enabled"`
}

// normalizeEgressIPv4CIDR 接受 IPv4 主机或 CIDR，主机规范为 /32。
func normalizeEgressIPv4CIDR(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("empty")
	}
	if err := ValidateIPv4OrCIDR(s); err != nil {
		return "", err
	}
	if !strings.Contains(s, "/") {
		return s + "/32", nil
	}
	if err := ValidateCIDR(s); err != nil {
		return "", err
	}
	return s, nil
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
	p.Match = strings.TrimSpace(strings.ToLower(p.Match))
	p.SrcCIDR = strings.TrimSpace(p.SrcCIDR)
	p.DstCIDR = strings.TrimSpace(p.DstCIDR)
	p.SrcAlias = strings.TrimSpace(p.SrcAlias)
	p.DstAlias = strings.TrimSpace(p.DstAlias)
	p.SrcIface = strings.TrimSpace(p.SrcIface)
	p.WanLinkID = strings.TrimSpace(p.WanLinkID)
	p.SNATIP = strings.TrimSpace(p.SNATIP)

	// 旧版 cidr+match 迁移到 src/dst
	if p.SrcCIDR == "" && p.DstCIDR == "" && p.SrcAlias == "" && p.DstAlias == "" && p.CIDR != "" {
		if p.Match == "destination" {
			p.DstCIDR = p.CIDR
		} else {
			p.SrcCIDR = p.CIDR
		}
	}
	if p.SrcCIDR == "" && p.DstCIDR == "" && p.SrcAlias == "" && p.DstAlias == "" && p.SrcIface == "" {
		return fmt.Errorf("source or destination (cidr, alias, or interface) required")
	}
	if p.SrcCIDR != "" && p.SrcAlias != "" {
		return fmt.Errorf("src_cidr and src_alias are mutually exclusive")
	}
	if p.DstCIDR != "" && p.DstAlias != "" {
		return fmt.Errorf("dst_cidr and dst_alias are mutually exclusive")
	}
	if p.SrcIface != "" {
		if err := ValidateIfaceName(p.SrcIface); err != nil {
			return fmt.Errorf("src_iface: %w", err)
		}
	}
	if p.SrcCIDR != "" {
		cidr, err := normalizeEgressIPv4CIDR(p.SrcCIDR)
		if err != nil {
			return fmt.Errorf("src_cidr: %w", err)
		}
		p.SrcCIDR = cidr
	}
	if p.DstCIDR != "" {
		cidr, err := normalizeEgressIPv4CIDR(p.DstCIDR)
		if err != nil {
			return fmt.Errorf("dst_cidr: %w", err)
		}
		p.DstCIDR = cidr
	}
	if p.SrcAlias != "" {
		if err := ValidateAliasName(p.SrcAlias); err != nil {
			return fmt.Errorf("src_alias: %w", err)
		}
	}
	if p.DstAlias != "" {
		if err := ValidateAliasName(p.DstAlias); err != nil {
			return fmt.Errorf("dst_alias: %w", err)
		}
	}
	if p.CIDR != "" {
		if err := ValidateCIDR(p.CIDR); err != nil {
			return err
		}
	}
	if p.WanLinkID == "" {
		return fmt.Errorf("wan_link_id required")
	}
	if p.Match != "" && p.Match != "source" && p.Match != "destination" {
		return fmt.Errorf("match must be source or destination")
	}
	if p.NoSNAT {
		p.SNATIP = ""
	} else if p.SNATIP != "" {
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
		if strings.TrimSpace(w.ID) != "" && w.Enabled {
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

// EgressPolicyCIDRs 返回 source 匹配出站策略的 CIDR（从主 WAN SNAT / 非对称回程中排除）。
// destination 匹配（如 Cloudflare CDN）由策略路由与专用 SNAT 规则处理，不应进入主 WAN 排除集。
func EgressPolicyCIDRs(policies []EgressPolicy) []string {
	return EgressPolicySourceMatchCIDRs(policies)
}

// EgressPolicySourceMatchCIDRs 仅 source 侧 CIDR（用于主 WAN SNAT 排除）；别名在运行时展开。
func EgressPolicySourceMatchCIDRs(policies []EgressPolicy) []string {
	return egressPolicySourceMatchCIDRs(policies, false)
}

// EgressPolicySnatSourceCIDRs 需要本机 SNAT 的 source CIDR（用于非对称回程丢弃；排除 no_snat）。
func EgressPolicySnatSourceCIDRs(policies []EgressPolicy) []string {
	return egressPolicySourceMatchCIDRs(policies, true)
}

func egressPolicySourceMatchCIDRs(policies []EgressPolicy, snatOnly bool) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range EnabledEgressPolicies(policies) {
		if snatOnly && p.NoSNAT {
			continue
		}
		for _, c := range []string{p.SrcCIDR, legacySourceCIDR(p)} {
			c = strings.TrimSpace(c)
			if c == "" || p.SrcAlias != "" {
				continue
			}
			if _, ok := seen[c]; ok {
				continue
			}
			seen[c] = struct{}{}
			out = append(out, c)
		}
	}
	return out
}

func legacySourceCIDR(p EgressPolicy) string {
	if p.SrcCIDR != "" || p.DstCIDR != "" || p.SrcAlias != "" || p.DstAlias != "" || p.SrcIface != "" {
		return ""
	}
	if p.Match == "destination" {
		return ""
	}
	return p.CIDR
}

// EgressSNATAddrPrefix 返回 nft SNAT 行使用的地址选择前缀（兼容旧 match 字段）。
func EgressSNATAddrPrefix(match string) string {
	if strings.TrimSpace(strings.ToLower(match)) == "destination" {
		return "ip daddr"
	}
	return "ip saddr"
}

// EgressSNATIPMatchClause 生成 nft SNAT 的 IP 匹配子句（不含 iifname）。
func EgressSNATIPMatchClause(p EgressPolicy) string {
	var parts []string
	if p.SrcAlias != "" {
		parts = append(parts, "ip saddr @alias_"+p.SrcAlias)
	} else if c := strings.TrimSpace(p.SrcCIDR); c != "" {
		parts = append(parts, "ip saddr "+c)
	}
	if p.DstAlias != "" {
		parts = append(parts, "ip daddr @alias_"+p.DstAlias)
	} else if c := strings.TrimSpace(p.DstCIDR); c != "" {
		parts = append(parts, "ip daddr "+c)
	}
	if len(parts) == 0 && p.CIDR != "" {
		parts = append(parts, EgressSNATAddrPrefix(p.Match)+" "+p.CIDR)
	}
	return strings.Join(parts, " ")
}

// EgressSNATMatchClause 生成 nft SNAT 匹配子句（源/目的 CIDR、别名与入接口）。
func EgressSNATMatchClause(p EgressPolicy) string {
	var parts []string
	if iif := strings.TrimSpace(p.SrcIface); iif != "" {
		parts = append(parts, fmt.Sprintf(`iifname "%s"`, iif))
	}
	if ip := EgressSNATIPMatchClause(p); ip != "" {
		parts = append(parts, ip)
	}
	return strings.Join(parts, " ")
}

// EgressPolicySignature 用于去重比较。
func EgressPolicySignature(p EgressPolicy) string {
	noSNAT := "0"
	if p.NoSNAT {
		noSNAT = "1"
	}
	return strings.Join([]string{
		p.SrcCIDR, p.DstCIDR, p.SrcAlias, p.DstAlias, p.SrcIface, p.CIDR, p.Match, p.WanLinkID, noSNAT,
	}, "|")
}

// ValidateEgressPolicyAliases 校验引用的别名存在且有成员。
func ValidateEgressPolicyAliases(p EgressPolicy, aliases []AliasSet) error {
	byName := AliasByName(aliases)
	for _, field := range []struct {
		name, val string
	}{
		{"src_alias", p.SrcAlias},
		{"dst_alias", p.DstAlias},
	} {
		if field.val == "" {
			continue
		}
		a, ok := byName[field.val]
		if !ok {
			return fmt.Errorf("%s: alias %q not found", field.name, field.val)
		}
		if len(a.Members) == 0 {
			return fmt.Errorf("%s: alias %q has no members", field.name, field.val)
		}
	}
	return nil
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
	Policy     EgressPolicy `json:"policy"`
	WanLink    WanLink      `json:"wan_link"`
	Device     string       `json:"device"`
	Gateway    string       `json:"gateway"`
	Table      int          `json:"table"`
	SNATIP     string       `json:"snat_ip"`
	Masquerade bool         `json:"masquerade,omitempty"`
	NoSNAT     bool         `json:"no_snat,omitempty"`
	Priority   int          `json:"priority"`
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
		if dev == "" {
			continue
		}
		tbl := WanLinkRouteTable(w.ID, st.Network.WanLinks)
		if tbl == 0 {
			continue
		}
		if p.NoSNAT {
			if gw == "" {
				continue
			}
			out = append(out, ResolvedEgress{
				Policy:   p,
				WanLink:  w,
				Device:   dev,
				Gateway:  gw,
				Table:    tbl,
				NoSNAT:   true,
				Priority: p.Priority,
			})
			continue
		}
		snat := p.SNATIP
		if snat == "" && primaryIP != nil {
			snat, _ = primaryIP(dev)
		}
		masquerade := false
		if snat == "" {
			// WARP 托管链路通常没有稳定的公网 IPv4，回退为按出口口 MASQUERADE。
			if IsWarpWanLink(w) {
				masquerade = true
			} else {
				continue
			}
		}
		out = append(out, ResolvedEgress{
			Policy:     p,
			WanLink:    w,
			Device:     dev,
			Gateway:    gw,
			Table:      tbl,
			SNATIP:     snat,
			Masquerade: masquerade,
			Priority:   p.Priority,
		})
	}
	return out
}

// CloudflareCDNCIDRsV4 Cloudflare 官方公布的 IPv4 CDN 网段（用于策略出口预置）
func CloudflareCDNCIDRsV4() []string {
	return []string{
		"173.245.48.0/20",
		"103.21.244.0/22",
		"103.22.200.0/22",
		"103.31.4.0/22",
		"141.101.64.0/18",
		"108.162.192.0/18",
		"190.93.240.0/20",
		"188.114.96.0/20",
		"197.234.240.0/22",
		"198.41.128.0/17",
		"162.158.0.0/15",
		"104.16.0.0/13",
		"104.24.0.0/14",
		"172.64.0.0/13",
		"131.0.72.0/22",
	}
}
