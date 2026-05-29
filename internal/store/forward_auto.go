package store

import (
	"fmt"
	"net"
	"strings"
)

const autoFwdIDPrefix = "auto-fwd-"

// AutoForwardRuleID 端口转发关联的防火墙规则 ID（按协议展开）。
func AutoForwardRuleID(forwardID, proto string) string {
	return fmt.Sprintf("%s%s-%s", autoFwdIDPrefix, forwardID, proto)
}

// IsAutoForwardRule 是否为端口转发自动同步的防火墙规则。
func IsAutoForwardRule(r FilterRule) bool {
	return strings.HasPrefix(strings.TrimSpace(r.ID), autoFwdIDPrefix)
}

// BuildAutoForwardFilterRules 为每条端口转发生成 forward 链放行规则（与策略生命周期绑定）。
func BuildAutoForwardFilterRules(forwards []WanPortForward, devLAN string) []FilterRule {
	devLAN = strings.TrimSpace(devLAN)
	if devLAN == "" || len(forwards) == 0 {
		return nil
	}
	var out []FilterRule
	for _, f := range forwards {
		iface := strings.TrimSpace(f.Interface)
		if iface == "" {
			continue
		}
		comment := strings.TrimSpace(f.Comment)
		if comment == "" {
			comment = f.ID
		}
		src := strings.TrimSpace(f.SrcAddr)
		for _, proto := range ForwardProtos(f.Proto) {
			r := FilterRule{
				ID:        AutoForwardRuleID(f.ID, proto),
				Chain:     "forward",
				Action:    "accept",
				Iif:       iface,
				Oif:       devLAN,
				Proto:     proto,
				DstAddr:   f.RedirectIP,
				DstPort:   f.RedirectPort,
				Comment:   fmt.Sprintf("端口转发 %s（自动）", comment),
				Enabled:   true,
				System:    true,
				IPVersion: f.IPVersion,
			}
			if !IsAnyCIDR(src) {
				r.SrcAddr = src
			}
			out = append(out, r)
		}
	}
	return out
}

// HairpinMatchAddr 回流 DNAT 匹配的目的地址（接口公网 IP 或策略指定的 dst_addr）。
func HairpinMatchAddr(f WanPortForward, iface string, primaryIPv4, primaryIPv6 func(string) (string, error)) string {
	dst := strings.TrimSpace(f.DstAddr)
	if dst != "" && !IsAnyCIDR(dst) {
		if ip, _, err := net.ParseCIDR(dst); err == nil {
			return ip.String()
		}
		if ip := net.ParseIP(dst); ip != nil {
			return ip.String()
		}
	}
	if f.IPVersion == "ipv6" && primaryIPv6 != nil {
		ip, err := primaryIPv6(iface)
		if err == nil && ip != "" {
			return ip
		}
		return ""
	}
	if primaryIPv4 != nil {
		ip, err := primaryIPv4(iface)
		if err == nil && ip != "" {
			return ip
		}
	}
	return ""
}
