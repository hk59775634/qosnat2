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
	return ApplyIFBMirredOnDevice(devLAN, cidrs)
}

// ApplyIFBMirredOnDevice 在指定接口 ingress 上匹配源网段，mirred 重定向到 ifb0（WireGuard 隧道上行等）
func ApplyIFBMirredOnDevice(dev string, cidrs []string) error {
	if dev == "" {
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
		delIFBMirredU32(dev, cidr)
		out, err := exec.Command("tc", "filter", "add", "dev", dev, "ingress",
			"protocol", "ip", "prio", "10", "u32",
			"match", "ip", "src", cidr,
			"action", "mirred", "egress", "redirect", "dev", ifbDev).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if !strings.Contains(msg, "File exists") {
				return fmt.Errorf("ifb mirred %s %s: %s %w", dev, cidr, msg, err)
			}
		}
	}
	return nil
}

// ApplyIFBMirredOnDeviceBidirectional 在 ingress 上同时对 src 与 dst 匹配 mirred（WG 客户端下行 dst 为本机隧道）
func ApplyIFBMirredOnDeviceBidirectional(dev string, cidrs []string) error {
	if dev == "" {
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
		delIFBMirredU32(dev, cidr)
		delIFBMirredDstU32(dev, cidr)
		out, err := exec.Command("tc", "filter", "add", "dev", dev, "ingress",
			"protocol", "ip", "prio", "10", "u32",
			"match", "ip", "src", cidr,
			"action", "mirred", "egress", "redirect", "dev", ifbDev).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if !strings.Contains(msg, "File exists") {
				return fmt.Errorf("ifb mirred src %s %s: %s %w", dev, cidr, msg, err)
			}
		}
		out, err = exec.Command("tc", "filter", "add", "dev", dev, "ingress",
			"protocol", "ip", "prio", "11", "u32",
			"match", "ip", "dst", cidr,
			"action", "mirred", "egress", "redirect", "dev", ifbDev).CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(out))
			if !strings.Contains(msg, "File exists") {
				return fmt.Errorf("ifb mirred dst %s %s: %s %w", dev, cidr, msg, err)
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

func delIFBMirredDstU32(devLAN, cidr string) {
	for i := 0; i < 8; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
			"protocol", "ip", "u32", "match", "ip", "dst", cidr).CombinedOutput()
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
	for _, prio := range []string{"10", "11"} {
		for i := 0; i < 64; i++ {
			out, _ := exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
				"protocol", "ip", "prio", prio, "u32").CombinedOutput()
			msg := string(out)
			if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") ||
				strings.Contains(msg, "does not match") {
				break
			}
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

// ResetIFBMirredOnDevice 清空设备 ingress 上 prio 10/11 的 u32 mirred 后重装（无 flower 清理，供 wg 等非 LAN 口）。
// clientBidirectionalMirred 为 true 时（WG 客户端）同时按 src 与 dst 匹配，使解密后下行（dst 为本机隧道）进入 IFB。
func ResetIFBMirredOnDevice(dev string, cidrs []string, clientBidirectionalMirred bool) error {
	if dev == "" {
		return nil
	}
	flushIngressMirredU32(dev)
	if len(cidrs) == 0 {
		return nil
	}
	if clientBidirectionalMirred {
		return ApplyIFBMirredOnDeviceBidirectional(dev, cidrs)
	}
	return ApplyIFBMirredOnDevice(dev, cidrs)
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

// ClearIFBMirred 删除 mirred 规则
func ClearIFBMirred(devLAN string, cidrs []string) {
	if devLAN == "" {
		return
	}
	for _, cidr := range cidrs {
		c := strings.TrimSpace(cidr)
		delIFBMirredU32(devLAN, c)
		delIFBMirredDstU32(devLAN, c)
	}
}
