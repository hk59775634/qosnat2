package store

import (
	"fmt"
	"strings"
)

const autoFwdLVSIDPrefix = "auto-fwd-lvs-"

// AutoLVSForwardRuleID LVS NAT 转发至 Real Server 的 forward 链规则 ID。
func AutoLVSForwardRuleID(vsID, rsIP, proto string) string {
	rsIP = strings.ReplaceAll(strings.TrimSpace(rsIP), ".", "-")
	return fmt.Sprintf("%s%s-%s-%s", autoFwdLVSIDPrefix, strings.TrimSpace(vsID), proto, rsIP)
}

// IsAutoLVSForwardRule 是否为 LVS 自动同步的 forward 规则。
func IsAutoLVSForwardRule(r FilterRule) bool {
	return strings.HasPrefix(strings.TrimSpace(r.ID), autoFwdLVSIDPrefix)
}

// BuildAutoLVSForwardFilterRules 为 LVS NAT 生成 WAN→LAN 至各 RS 的 forward 放行（与端口转发 auto-fwd 类似）。
func BuildAutoLVSForwardFilterRules(l LVSState, devLAN, defaultWAN string) []FilterRule {
	if !l.Enabled {
		return nil
	}
	devLAN = strings.TrimSpace(devLAN)
	if devLAN == "" {
		return nil
	}
	var out []FilterRule
	for _, vs := range l.VirtualServers {
		iif := strings.TrimSpace(vs.WANDevice)
		if iif == "" {
			iif = strings.TrimSpace(defaultWAN)
		}
		if iif == "" {
			continue
		}
		label := strings.TrimSpace(vs.Comment)
		if label == "" {
			label = vs.VIP
		}
		for _, rs := range vs.RealServers {
			port := rs.Port
			if port <= 0 {
				port = vs.Port
			}
			for _, proto := range LVSProtos(vs.Protocol) {
				out = append(out, FilterRule{
					ID:        AutoLVSForwardRuleID(vs.ID, rs.IP, proto),
					Chain:     "forward",
					Action:    "accept",
					Iif:       iif,
					Oif:       devLAN,
					Proto:     proto,
					DstAddr:   rs.IP + "/32",
					DstPort:   port,
					Comment:   fmt.Sprintf("LVS %s → RS %s %s/%d（自动）", label, rs.IP, strings.ToUpper(proto), port),
					Enabled:   true,
					System:    true,
					IPVersion: "ipv4",
				})
			}
		}
	}
	return out
}
