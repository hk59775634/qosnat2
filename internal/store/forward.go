package store

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
)

// WanPortForward WAN 端口转发 / DNAT
type WanPortForward struct {
	ID           string `json:"id"`
	Interface    string `json:"interface"`              // 入接口 iifname，空则 DEV_WAN
	IPVersion    string `json:"ip_version"`             // ipv4 | ipv6
	Proto        string `json:"proto"`                  // tcp | udp | tcp_udp
	SrcAddr      string `json:"src_addr"`               // 源地址 CIDR，默认 0.0.0.0/0 或 ::/0
	DstAddr      string `json:"dst_addr,omitempty"`     // 目标地址（接口 IP），空表示任意
	DstPort      int    `json:"dst_port"`               // 目标端口（公网监听）
	RedirectIP   string `json:"redirect_ip"`            // 重定向目标 IP
	RedirectPort int    `json:"redirect_port"`          // 重定向目标端口
	Comment      string `json:"comment,omitempty"`      // 描述
	WanPort      int    `json:"wan_port,omitempty"`     // 已废弃 → dst_port
	HostIP       string `json:"host_ip,omitempty"`      // 已废弃 → redirect_ip
	HostPort     int    `json:"host_port,omitempty"`    // 已废弃 → redirect_port
}

// NormalizeWanForward 校验并填充默认值
func NormalizeWanForward(f *WanPortForward, defaultWAN string) error {
	if f == nil {
		return fmt.Errorf("forward nil")
	}
	migrateLegacyForward(f)
	if f.ID == "" {
		f.ID = NewForwardID()
	}
	iface := strings.TrimSpace(f.Interface)
	if iface == "" {
		iface = strings.TrimSpace(defaultWAN)
	}
	f.Interface = iface
	if iface == "" && defaultWAN != "" {
		return fmt.Errorf("interface required")
	}
	ver := strings.ToLower(strings.TrimSpace(f.IPVersion))
	if ver == "" {
		ver = "ipv4"
	}
	if ver != "ipv4" && ver != "ipv6" {
		return fmt.Errorf("ip_version must be ipv4 or ipv6")
	}
	f.IPVersion = ver
	proto := strings.ToLower(strings.TrimSpace(f.Proto))
	switch proto {
	case "", "tcp":
		f.Proto = "tcp"
	case "udp":
		f.Proto = "udp"
	case "tcp_udp", "tcp+udp", "both":
		f.Proto = "tcp_udp"
	default:
		return fmt.Errorf("proto must be tcp, udp, or tcp_udp")
	}
	src := strings.TrimSpace(f.SrcAddr)
	if src == "" {
		if ver == "ipv6" {
			src = "::/0"
		} else {
			src = "0.0.0.0/0"
		}
	}
	if _, _, err := net.ParseCIDR(src); err != nil {
		parsed := net.ParseIP(src)
		if parsed == nil {
			return fmt.Errorf("invalid src_addr: %q", f.SrcAddr)
		}
		if parsed.To4() != nil {
			src += "/32"
		} else {
			src += "/128"
		}
	}
	f.SrcAddr = src
	dst := strings.TrimSpace(f.DstAddr)
	if dst != "" {
		if _, _, err := net.ParseCIDR(dst); err != nil {
			parsed := net.ParseIP(dst)
			if parsed == nil {
				return fmt.Errorf("invalid dst_addr: %q", f.DstAddr)
			}
			if parsed.To4() != nil {
				dst += "/32"
			} else {
				dst += "/128"
			}
		}
		f.DstAddr = dst
	}
	if f.DstPort <= 0 || f.DstPort > 65535 {
		return fmt.Errorf("dst_port required (1-65535)")
	}
	rip := strings.TrimSpace(f.RedirectIP)
	if rip == "" {
		return fmt.Errorf("redirect_ip required")
	}
	parsed := net.ParseIP(rip)
	if parsed == nil {
		return fmt.Errorf("invalid redirect_ip: %q", rip)
	}
	if ver == "ipv4" && parsed.To4() == nil {
		return fmt.Errorf("redirect_ip must be ipv4")
	}
	if ver == "ipv6" && parsed.To4() != nil {
		return fmt.Errorf("redirect_ip must be ipv6")
	}
	f.RedirectIP = rip
	if f.RedirectPort <= 0 || f.RedirectPort > 65535 {
		if f.RedirectPort == 0 {
			f.RedirectPort = f.DstPort
		} else {
			return fmt.Errorf("redirect_port invalid")
		}
	}
	f.Comment = strings.TrimSpace(f.Comment)
	return nil
}

func migrateLegacyForward(f *WanPortForward) {
	if f.DstPort == 0 && f.WanPort > 0 {
		f.DstPort = f.WanPort
	}
	if f.RedirectIP == "" && f.HostIP != "" {
		f.RedirectIP = f.HostIP
	}
	if f.RedirectPort == 0 && f.HostPort > 0 {
		f.RedirectPort = f.HostPort
	}
	f.WanPort = 0
	f.HostIP = ""
	f.HostPort = 0
}

// ForwardProtos 展开 tcp_udp
func ForwardProtos(proto string) []string {
	if strings.ToLower(proto) == "tcp_udp" {
		return []string{"tcp", "udp"}
	}
	p := strings.ToLower(proto)
	if p == "" {
		return []string{"tcp"}
	}
	return []string{p}
}

// IsAnyCIDR 是否为任意源/目的
func IsAnyCIDR(cidr string) bool {
	cidr = strings.TrimSpace(cidr)
	return cidr == "" || cidr == "0.0.0.0/0" || cidr == "::/0"
}

// NewForwardID 生成规则 ID
func NewForwardID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return "fwd-" + hex.EncodeToString(b[:])
}

// MigrateWanForwards 启动时迁移旧字段并补 id
func MigrateWanForwards(forwards *[]WanPortForward) {
	for i := range *forwards {
		migrateLegacyForward(&(*forwards)[i])
		if (*forwards)[i].ID == "" {
			(*forwards)[i].ID = NewForwardID()
		}
		if (*forwards)[i].IPVersion == "" {
			(*forwards)[i].IPVersion = "ipv4"
		}
		if (*forwards)[i].Proto == "" {
			(*forwards)[i].Proto = "tcp"
		}
		if (*forwards)[i].SrcAddr == "" {
			(*forwards)[i].SrcAddr = "0.0.0.0/0"
		}
	}
}
