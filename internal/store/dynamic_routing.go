package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// DynamicRoutingState FRR 动态路由（BGP/OSPF），仅在 route_backend=frr 时使用。
type DynamicRoutingState struct {
	BGP  BGPConfig  `json:"bgp,omitempty"`
	OSPF OSPFConfig `json:"ospf,omitempty"`
}

// BGPConfig BGP 实例配置。
type BGPConfig struct {
	Enabled               bool          `json:"enabled"`
	ASN                   uint32        `json:"asn,omitempty"`
	RouterID              string        `json:"router_id,omitempty"`
	Neighbors             []BGPNeighbor `json:"neighbors,omitempty"`
	Networks              []string      `json:"networks,omitempty"`
	RedistributeConnected bool          `json:"redistribute_connected,omitempty"`
}

// BGPNeighbor BGP 对等体。
type BGPNeighbor struct {
	ID           string `json:"id"`
	Address      string `json:"address"`
	RemoteASN    uint32 `json:"remote_asn"`
	Description  string `json:"description,omitempty"`
	UpdateSource string `json:"update_source,omitempty"`
	Password     string `json:"password,omitempty"`
	Enabled      bool   `json:"enabled"`
}

// OSPFConfig OSPF 实例配置。
type OSPFConfig struct {
	Enabled               bool              `json:"enabled"`
	RouterID              string            `json:"router_id,omitempty"`
	Networks              []OSPFNetwork     `json:"networks,omitempty"`
	RedistributeConnected bool              `json:"redistribute_connected,omitempty"`
}

// OSPFNetwork OSPF 宣告网段。
type OSPFNetwork struct {
	ID      string `json:"id"`
	Prefix  string `json:"prefix"`
	Area    string `json:"area,omitempty"`
	Enabled bool   `json:"enabled"`
}

func NewBGPNeighborID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "bgp-n-" + hex.EncodeToString(b[:])
}

func NewOSPFNetworkID() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return "ospf-n-" + hex.EncodeToString(b[:])
}

// NormalizeDynamicRouting 校验并规范化动态路由配置。
func NormalizeDynamicRouting(dr *DynamicRoutingState) error {
	if dr == nil {
		return fmt.Errorf("dynamic routing nil")
	}
	if err := normalizeBGPConfig(&dr.BGP); err != nil {
		return fmt.Errorf("bgp: %w", err)
	}
	if err := normalizeOSPFConfig(&dr.OSPF); err != nil {
		return fmt.Errorf("ospf: %w", err)
	}
	return nil
}

func normalizeBGPConfig(b *BGPConfig) error {
	if b == nil {
		return fmt.Errorf("config nil")
	}
	b.RouterID = strings.TrimSpace(b.RouterID)
	if b.Enabled {
		if b.ASN == 0 || b.ASN > 4294967295 {
			return fmt.Errorf("asn required when enabled")
		}
		if b.RouterID != "" {
			if err := validateRouterID(b.RouterID); err != nil {
				return err
			}
		}
	}
	seen := map[string]struct{}{}
	for i := range b.Neighbors {
		n := &b.Neighbors[i]
		if n.ID == "" {
			n.ID = NewBGPNeighborID()
		}
		n.Address = strings.TrimSpace(n.Address)
		n.Description = strings.TrimSpace(n.Description)
		n.UpdateSource = strings.TrimSpace(n.UpdateSource)
		n.Password = strings.TrimSpace(n.Password)
		if !n.Enabled {
			continue
		}
		if n.Address == "" {
			return fmt.Errorf("neighbor %s: address required", n.ID)
		}
		if err := validateRouterID(n.Address); err != nil {
			return fmt.Errorf("neighbor %s address: %w", n.ID, err)
		}
		if n.RemoteASN == 0 {
			return fmt.Errorf("neighbor %s: remote_asn required", n.ID)
		}
		if n.UpdateSource != "" {
			if err := validateRouterID(n.UpdateSource); err != nil {
				return fmt.Errorf("neighbor %s update_source: %w", n.ID, err)
			}
		}
		if _, dup := seen[n.Address]; dup {
			return fmt.Errorf("duplicate neighbor address %s", n.Address)
		}
		seen[n.Address] = struct{}{}
	}
	var nets []string
	for _, cidr := range b.Networks {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if err := ValidateCIDR(cidr); err != nil {
			return fmt.Errorf("network %s: %w", cidr, err)
		}
		nets = append(nets, cidr)
	}
	b.Networks = nets
	return nil
}

func normalizeOSPFConfig(o *OSPFConfig) error {
	if o == nil {
		return fmt.Errorf("config nil")
	}
	o.RouterID = strings.TrimSpace(o.RouterID)
	if o.Enabled && o.RouterID != "" {
		if err := validateRouterID(o.RouterID); err != nil {
			return err
		}
	}
	for i := range o.Networks {
		n := &o.Networks[i]
		if n.ID == "" {
			n.ID = NewOSPFNetworkID()
		}
		n.Prefix = strings.TrimSpace(n.Prefix)
		n.Area = normalizeOSPFArea(n.Area)
		if !n.Enabled {
			continue
		}
		if n.Prefix == "" {
			return fmt.Errorf("network %s: prefix required", n.ID)
		}
		if err := ValidateCIDR(n.Prefix); err != nil {
			return fmt.Errorf("network %s: %w", n.ID, err)
		}
		if n.Area == "" {
			n.Area = "0.0.0.0"
		}
	}
	return nil
}

func validateRouterID(s string) error {
	if err := ValidateIPv4OrCIDR(s); err != nil {
		return err
	}
	if strings.Contains(s, "/") {
		return fmt.Errorf("must be IPv4 address, not CIDR")
	}
	return nil
}

func normalizeOSPFArea(area string) string {
	area = strings.TrimSpace(area)
	if area == "" {
		return ""
	}
	if area == "0" || area == "0.0.0.0" {
		return "0.0.0.0"
	}
	if _, err := strconv.ParseUint(area, 10, 32); err == nil {
		return area
	}
	if ip := net.ParseIP(area); ip != nil && ip.To4() != nil {
		return area
	}
	return area
}

// DynamicRoutingNeedsBGP 是否应启用 bgpd。
func DynamicRoutingNeedsBGP(dr DynamicRoutingState) bool {
	return dr.BGP.Enabled
}

// DynamicRoutingNeedsOSPF 是否应启用 ospfd。
func DynamicRoutingNeedsOSPF(dr DynamicRoutingState) bool {
	return dr.OSPF.Enabled
}
