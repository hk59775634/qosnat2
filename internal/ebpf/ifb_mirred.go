package ebpf

import (
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"sort"
	"strings"
)

func sortCIDRsByPrefixLen(cidrs []string) []string {
	type item struct {
		cidr string
		ones int
	}
	var list []item
	seen := map[string]struct{}{}
	for _, c := range cidrs {
		c = strings.TrimSpace(c)
		if c == "" {
			continue
		}
		if _, ok := seen[c]; ok {
			continue
		}
		seen[c] = struct{}{}
		ones := 0
		if _, n, err := net.ParseCIDR(c); err == nil && n != nil {
			ones, _ = n.Mask.Size()
		}
		list = append(list, item{cidr: c, ones: ones})
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ones > list[j].ones
	})
	out := make([]string, len(list))
	for i, it := range list {
		out[i] = it.cidr
	}
	return out
}

// ApplyIFBMirred 将 LAN ingress 匹配源网段的上行导入 ifb0（u32+mirred，比 flower 对本机目的更可靠）
func ApplyIFBMirred(devLAN string, cidrs []string) error {
	if devLAN == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for _, cidr := range sortCIDRsByPrefixLen(cidrs) {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		delIFBMirredU32(devLAN, cidr)
		out, err := exec.Command("tc", "filter", "add", "dev", devLAN, "ingress",
			"protocol", "ip", "prio", "10", "u32",
			"match", "ip", "src", cidr,
			"action", "mirred", "egress", "redirect", "dev", ifbDev).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if !strings.Contains(msg, "File exists") {
				return fmt.Errorf("ifb mirred %s %s: %s %w", devLAN, cidr, msg, err)
			}
		}
	}
	return nil
}

var flowerSrcIPRe = regexp.MustCompile(`src_ip ([0-9a-fA-F./:]+)`)

func delIFBMirredU32(devLAN, cidr string) {
	for i := 0; i < 8; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
			"protocol", "ip", "u32", "match", "ip", "src", cidr).CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") {
			break
		}
	}
}

func delFlowerMirred(devLAN, cidr string) {
	_ = exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
		"protocol", "ip", "flower", "src_ip", cidr).Run()
}

// delAllFlowerIngress 删除遗留 flower mirred（须带 src_ip，否则内核拒绝 flush）
func delAllFlowerIngress(devLAN string, knownCIDRs []string) {
	seen := map[string]struct{}{}
	for _, c := range knownCIDRs {
		c = strings.TrimSpace(c)
		if c != "" {
			seen[c] = struct{}{}
			delFlowerMirred(devLAN, c)
		}
	}
	out, _ := exec.Command("tc", "filter", "show", "dev", devLAN, "ingress").CombinedOutput()
	if !strings.Contains(string(out), "flower") {
		return
	}
	for _, m := range flowerSrcIPRe.FindAllStringSubmatch(string(out), -1) {
		if len(m) < 2 {
			continue
		}
		cidr := strings.TrimSpace(m[1])
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		delFlowerMirred(devLAN, cidr)
	}
}

func flushIngressMirredU32(devLAN string) {
	for i := 0; i < 64; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
			"protocol", "ip", "prio", "10", "u32").CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") ||
			strings.Contains(msg, "does not match") {
			break
		}
	}
}

// ResetIFBMirred 清空 LAN ingress mirred（flower→u32 迁移 + 去重）后按当前 cidrs 重装
func ResetIFBMirred(devLAN string, cidrs []string) error {
	if devLAN == "" {
		return nil
	}
	delAllFlowerIngress(devLAN, cidrs)
	flushIngressMirredU32(devLAN)
	return ApplyIFBMirred(devLAN, cidrs)
}

// ApplyIFBIngressBPF 在 ifb0 ingress 挂分类器（mirred 入 ifb 后更新 map/统计）
func ApplyIFBIngressBPF(ingressPin string) error {
	if ingressPin == "" {
		return nil
	}
	_ = exec.Command("tc", "filter", "del", "dev", ifbDev, "ingress",
		"protocol", "all", "prio", "1").Run()
	out, err := exec.Command("tc", "filter", "add", "dev", ifbDev, "ingress",
		"protocol", "all", "prio", "1", "bpf",
		"direct-action", "object-pinned", ingressPin, "classid", "1:0").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			return fmt.Errorf("ifb ingress bpf: %s %w", msg, err)
		}
	}
	return nil
}

// RemoveLANIngressBPF 移除 LAN ingress BPF（direct-action 会阻止后续 u32 mirred）
func RemoveLANIngressBPF(devLAN string) {
	if devLAN == "" {
		return
	}
	_ = exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
		"protocol", "all", "prio", "1").Run()
}

// ApplyLANIngressBPF 已弃用：分类改在 ifb0 ingress（mirred 之后）
func ApplyLANIngressBPF(devLAN, ingressPin string) error {
	if devLAN == "" || ingressPin == "" {
		return nil
	}
	_ = exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
		"protocol", "all", "prio", "1").Run()
	out, err := exec.Command("tc", "filter", "add", "dev", devLAN, "ingress",
		"protocol", "all", "prio", "1", "bpf",
		"direct-action", "object-pinned", ingressPin, "classid", "1:0").CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if !strings.Contains(msg, "File exists") {
			return fmt.Errorf("lan ingress bpf %s: %s %w", devLAN, msg, err)
		}
	}
	return nil
}

// ClearIFBMirred 删除 mirred 规则
func ClearIFBMirred(devLAN string, cidrs []string) {
	if devLAN == "" {
		return
	}
	for _, cidr := range cidrs {
		delIFBMirredU32(devLAN, strings.TrimSpace(cidr))
	}
}
