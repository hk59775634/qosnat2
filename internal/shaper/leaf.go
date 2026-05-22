package shaper

import (
	"strconv"
	"strings"
)

// AllowedLeafInput API 请求中的 leaf（空表示不修改；历史 fq 仍接受）
func AllowedLeafInput(raw string) bool {
	raw = strings.ToLower(strings.TrimSpace(raw))
	if raw == "" {
		return true
	}
	return raw == "fq_codel" || raw == "cake" || raw == "fq"
}

// ValidLeaf 支持的 HTB 叶子 qdisc
func ValidLeaf(leaf string) bool {
	switch strings.ToLower(strings.TrimSpace(leaf)) {
	case "", "fq_codel", "cake":
		return true
	default:
		return false
	}
}

// NormalizeLeaf 默认 fq_codel
func NormalizeLeaf(leaf string) string {
	leaf = strings.ToLower(strings.TrimSpace(leaf))
	if leaf == "" {
		return "fq_codel"
	}
	if leaf == "fq" {
		return "fq_codel"
	}
	if ValidLeaf(leaf) {
		return leaf
	}
	return "fq_codel"
}

// LeafModules 需要 modprobe 的模块
func LeafModules(leaf string) []string {
	base := []string{"ifb", "sch_htb", "cls_bpf", "act_bpf", "act_mirred"}
	leaf = NormalizeLeaf(leaf)
	switch leaf {
	case "cake":
		return append(base, "sch_cake")
	default:
		return append(base, "sch_fq_codel")
	}
}

// FQOpts fq_codel 可选 flows、quantum（0 表示默认；cake 忽略）
type FQOpts struct {
	Flows   int
	Quantum int
}

// LeafTCArgs tc qdisc add 在 parent 之后的参数（不含 tc/qdisc/add/dev/parent）
func LeafTCArgs(leaf string, fq FQOpts) []string {
	leaf = NormalizeLeaf(leaf)
	switch leaf {
	case "fq_codel":
		args := []string{leaf, "limit", "10240"}
		return appendFQOpts(args, fq)
	case "cake":
		return []string{leaf, "besteffort"}
	default:
		args := []string{"fq_codel", "limit", "10240"}
		return appendFQOpts(args, fq)
	}
}

func appendFQOpts(args []string, fq FQOpts) []string {
	if fq.Flows > 0 {
		args = append(args, "flows", strconv.Itoa(fq.Flows))
	}
	if fq.Quantum > 0 {
		args = append(args, "quantum", strconv.Itoa(fq.Quantum))
	}
	return args
}
