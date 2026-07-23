package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// RouteNexthop 多路径 default 的一跳（ip route replace default nexthop … weight …）
type RouteNexthop struct {
	Gateway string `json:"gateway"`
	Device  string `json:"device,omitempty"`
	Weight  int    `json:"weight,omitempty"`
}

const (
	RouteSourceManual = "manual"
	RouteSourceWan    = "wan"
	RouteSourceEgress = "egress"
)

// RouteEntry 由 qosnatd 管理的静态路由（ip route replace）
type RouteEntry struct {
	ID         string         `json:"id"`
	Dest       string         `json:"dest"` // CIDR 或 default
	Gateway    string         `json:"gateway,omitempty"`
	Device     string         `json:"device,omitempty"`
	Nexthops   []RouteNexthop `json:"nexthops,omitempty"`
	Table      int            `json:"table,omitempty"` // 0/254 = main
	Metric     int            `json:"metric,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Comment    string         `json:"comment,omitempty"`
	Enabled    bool           `json:"enabled"`
	Source     string         `json:"source,omitempty"`      // manual | wan | egress
	SourceNote string         `json:"source_note,omitempty"` // 只读说明，便于区分相似 default 路由
	Locked     bool           `json:"locked,omitempty"`      // 由多 WAN / 出站策略同步，不可在本页删除或编辑
}

// IsAutoManagedRoute 是否为多 WAN 或策略出站自动同步的路由
func IsAutoManagedRoute(r RouteEntry) bool {
	if r.Locked {
		return true
	}
	return strings.HasPrefix(r.Comment, wanRouteCommentPrefix) ||
		strings.HasPrefix(r.Comment, egressRouteCommentPrefix) ||
		strings.HasPrefix(r.Comment, wanPolicyRouteCommentPrefix)
}

// NewRouteID 生成路由条目 ID
func NewRouteID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "rt-" + hex.EncodeToString(b[:])
}

// NormalizeRouteDest 规范化目标：default、裸 IPv4→/32、裸 IPv6→/128，便于与 ip -json 对齐。
func NormalizeRouteDest(dest string) (string, error) {
	dest = trim(dest)
	if dest == "" || dest == "0.0.0.0/0" || dest == "::/0" {
		return "default", nil
	}
	if ip := net.ParseIP(dest); ip != nil {
		if v4 := ip.To4(); v4 != nil {
			return v4.String() + "/32", nil
		}
		return ip.String() + "/128", nil
	}
	if ip, ipnet, err := net.ParseCIDR(dest); err == nil {
		ones, bits := ipnet.Mask.Size()
		if v4 := ip.To4(); v4 != nil && bits == 32 && ones == 32 {
			return v4.String() + "/32", nil
		}
		if bits == 128 && ones == 128 {
			return ip.String() + "/128", nil
		}
		return ipnet.String(), nil
	}
	return dest, nil
}

func trim(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// FindRoute 按 ID 查找
func FindRoute(routes []RouteEntry, id string) (RouteEntry, int, bool) {
	for i, r := range routes {
		if r.ID == id {
			return r, i, true
		}
	}
	return RouteEntry{}, -1, false
}

// RouteKey 用于匹配内核路由与托管项
func RouteKey(dest, gateway, device string, table int) string {
	if table == 0 {
		table = 254
	}
	return fmt.Sprintf("%s|%s|%s|%d", dest, gateway, device, table)
}
