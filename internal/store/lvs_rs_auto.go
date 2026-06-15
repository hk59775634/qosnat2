package store

import (
	"fmt"
	"strings"
)

const autoIDInputLVSRSPrefix = "auto-input-lvs-rs"

// BuildAutoLVSRSInputRules 为 DR Real Server 在 LAN input 放行 VIP:port。
func BuildAutoLVSRSInputRules(l LVSState, devLAN string) []FilterRule {
	if !l.Enabled || LVSRole(&l) != LVSRoleRS {
		return nil
	}
	devLAN = strings.TrimSpace(devLAN)
	if devLAN == "" {
		return nil
	}
	var out []FilterRule
	for _, e := range l.RS.Entries {
		label := strings.TrimSpace(e.Comment)
		if label == "" {
			label = e.VIP
		}
		id := strings.TrimSpace(e.ID)
		if id == "" {
			id = "rs"
		}
		for _, proto := range LVSProtos(e.Protocol) {
			out = append(out, FilterRule{
				ID:        fmt.Sprintf("%s-%s-%s-%s", autoIDInputLVSRSPrefix, id, proto, devLAN),
				Chain:     "input",
				Action:    "accept",
				Iif:       devLAN,
				Proto:     proto,
				DstAddr:   e.VIP + "/32",
				DstPort:   e.Port,
				Comment:   fmt.Sprintf("LVS RS %s %s/%d %s（自动）", label, strings.ToUpper(proto), e.Port, devLAN),
				Enabled:   true,
				System:    true,
				IPVersion: "ipv4",
			})
		}
	}
	return out
}
