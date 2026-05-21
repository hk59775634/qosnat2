package ebpf

import (
	"fmt"
	"os/exec"
	"strings"
)

// ApplyIFBMirred 将 LAN ingress 上匹配网段的上行流量 mirred 到 ifb0（与 BPF 分类器配合）
func ApplyIFBMirred(devLAN string, cidrs []string) error {
	if devLAN == "" {
		return nil
	}
	seen := map[string]struct{}{}
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		if _, ok := seen[cidr]; ok {
			continue
		}
		seen[cidr] = struct{}{}
		_ = exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
			"protocol", "ip", "flower", "src_ip", cidr).Run()
		out, err := exec.Command("tc", "filter", "add", "dev", devLAN, "ingress",
			"protocol", "ip", "prio", "10", "flower", "src_ip", cidr,
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

// ClearIFBMirred 删除 ApplyIFBMirred 添加的 flower 规则
func ClearIFBMirred(devLAN string, cidrs []string) {
	if devLAN == "" {
		return
	}
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		_ = exec.Command("tc", "filter", "del", "dev", devLAN, "ingress",
			"protocol", "ip", "flower", "src_ip", cidr).Run()
	}
}
