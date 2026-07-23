package wg

import (
	"fmt"
	"net"
	"strings"
	"unicode"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

const endpointRouteCommentPrefix = "qosnat-wg-ep:"

// SyncEndpointRoutes 为配置了静态 Endpoint 的 Peer 写入 /32|/128 主机路由，
// 避免握手/keepalive 被其它 Peer（AllowedIPs=0.0.0.0/0）或默认路由吸入隧道导致 endpoint 漫游。
func SyncEndpointRoutes(st *store.State) {
	if st == nil {
		return
	}
	keep := make([]store.RouteEntry, 0, len(st.Routes))
	for _, r := range st.Routes {
		if strings.HasPrefix(r.Comment, endpointRouteCommentPrefix) {
			continue
		}
		keep = append(keep, r)
	}
	seen := map[string]struct{}{}
	for _, r := range keep {
		if !r.Enabled {
			continue
		}
		dest, err := store.NormalizeRouteDest(r.Dest)
		if err != nil || dest == "" || dest == "default" {
			continue
		}
		seen[dest] = struct{}{}
	}
	for _, inst := range st.VPN.WireGuards {
		if !inst.Enabled {
			continue
		}
		wgDev := strings.TrimSpace(inst.Interface)
		if wgDev == "" {
			wgDev = "wg0"
		}
		for _, p := range inst.Peers {
			host := ParseEndpointHost(p.Endpoint)
			ip := net.ParseIP(host)
			if ip == nil {
				continue
			}
			dest := endpointDest(ip)
			ndest, err := store.NormalizeRouteDest(dest)
			if err != nil || ndest == "" {
				continue
			}
			if _, ok := seen[ndest]; ok {
				continue
			}
			gw, dev := netif.RouteGetNexthop(host)
			if dev == "" || isWGTunnelDevice(dev) || dev == wgDev {
				continue
			}
			seen[ndest] = struct{}{}
			id := "wg-ep-" + sanitizeID(strings.ReplaceAll(ndest, "/", "-"))
			note := fmt.Sprintf("WG Endpoint · %s · %s", wgDev, strings.TrimSpace(p.Name))
			keep = append(keep, store.RouteEntry{
				ID:         id,
				Dest:       ndest,
				Gateway:    gw,
				Device:     dev,
				Comment:    endpointRouteCommentPrefix + ndest,
				Enabled:    true,
				Source:     store.RouteSourceManual,
				Locked:     true,
				SourceNote: note,
			})
		}
	}
	st.Routes = keep
}

func endpointDest(ip net.IP) string {
	if v4 := ip.To4(); v4 != nil {
		return v4.String() + "/32"
	}
	return ip.String() + "/128"
}

func isWGTunnelDevice(dev string) bool {
	dev = strings.TrimSpace(dev)
	return strings.HasPrefix(dev, "wg") || strings.HasPrefix(dev, "qwp") || strings.HasPrefix(dev, "qpe")
}

func sanitizeID(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "x"
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r), r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	out := b.String()
	if out == "" {
		return "x"
	}
	return out
}
