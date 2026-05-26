package nft

import (
	"os"
	"os/exec"
	"strings"
)

// Mark 隔离常量（§7.5）
const (
	NftMarkAllowedMax = 0x0fffffff
	QosMarkReserved   = 0xf0000000
)

// MarkPolicy 文档化策略 + 审计结果
type MarkPolicy struct {
	NftAllowedMax uint32 `json:"nft_allowed_max"`
	QosReserved   uint32 `json:"qos_reserved"`
	IFBMethod     string `json:"ifb_method"`
	TCClassIDUse  string `json:"tc_classid_use"`
	RulesOK       bool   `json:"rules_ok"`
	Issues        []string `json:"issues,omitempty"`
	NftUsesMark   bool   `json:"nft_uses_skb_mark"`
}

// DefaultMarkPolicy 返回策略说明
func DefaultMarkPolicy() MarkPolicy {
	return MarkPolicy{
		NftAllowedMax: NftMarkAllowedMax,
		QosReserved:   QosMarkReserved,
		IFBMethod:     "bpf_redirect(ifb0) — does not use skb->mark",
		TCClassIDUse:  "BPF classify sets tc_classid → HTB",
		RulesOK:       true,
	}
}

// AuditMarkIsolation 检查 ruleset 是否违反 mark 隔离
func AuditMarkIsolation() MarkPolicy {
	p := DefaultMarkPolicy()
	var texts []string
	if b, err := os.ReadFile(RulesPath); err == nil {
		texts = append(texts, string(b))
	}
	if out, err := exec.Command("nft", "list", "ruleset").CombinedOutput(); err == nil {
		texts = append(texts, string(out))
	}
	for _, body := range texts {
		for _, line := range strings.Split(body, "\n") {
			trim := strings.TrimSpace(line)
			if trim == "" || strings.HasPrefix(trim, "#") {
				continue
			}
			low := strings.ToLower(trim)
			if strings.Contains(low, "flowtable") {
				p.RulesOK = false
				p.Issues = append(p.Issues, "forbidden flowtable: "+trim)
			}
			if strings.Contains(low, "meta mark set") {
				p.NftUsesMark = true
				p.RulesOK = false
				p.Issues = append(p.Issues, "meta mark set: "+trim)
			}
		}
	}
	if len(p.Issues) == 0 {
		p.RulesOK = true
	}
	return p
}
