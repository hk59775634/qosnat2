package shaper

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
)

// tc prio 越小越先匹配；/32 主机规则先于 /24 网段
const (
	ifbU32HostPrio = "100"
	ifbU32CIDRPrio = "200"
)

// installIFBUploadFilter 在 ifb0 egress HTB 上为单 IP 安装 u32 flowid
func installIFBUploadFilter(ip string, minor uint32) error {
	cid := fmt.Sprintf("1:%x", minor)
	_ = exec.Command("tc", "filter", "del", "dev", IFBDev, "parent", "1:",
		"protocol", "ip", "u32", "match", "ip", "src", ip+"/32").Run()
	out, err := exec.Command("tc", "filter", "add", "dev", IFBDev, "parent", "1:",
		"protocol", "ip", "prio", ifbU32HostPrio, "u32",
		"match", "ip", "src", ip+"/32",
		"flowid", cid).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "File exists") {
			return nil
		}
		return fmt.Errorf("ifb u32 %s flowid %s: %s %w", ip, cid, msg, err)
	}
	return nil
}

func removeIFBUploadFilter(ip string) error {
	_ = exec.Command("tc", "filter", "del", "dev", IFBDev, "parent", "1:",
		"protocol", "ip", "u32", "match", "ip", "src", ip+"/32").Run()
	return nil
}

func ifbSubnetPrio(cidr string) string {
	_, n, err := net.ParseCIDR(cidr)
	if err != nil || n == nil {
		return ifbU32CIDRPrio
	}
	ones, bits := n.Mask.Size()
	if ones == bits {
		return ifbU32HostPrio
	}
	// 前缀越长 prio 越小（越先匹配）：/25=175 /24=176
	p := 200 - ones
	if p < 101 {
		p = 101
	}
	return strconv.Itoa(p)
}

// installIFBUploadFilterCIDR 网段 profile 兜底 u32（/24 等在 per-IP 规则建立前即可走 ifb 整形）
func installIFBUploadFilterCIDR(cidr string, minor uint32) error {
	cid := fmt.Sprintf("1:%x", minor)
	prio := ifbSubnetPrio(cidr)
	_ = exec.Command("tc", "filter", "del", "dev", IFBDev, "parent", "1:",
		"protocol", "ip", "u32", "match", "ip", "src", cidr).Run()
	out, err := exec.Command("tc", "filter", "add", "dev", IFBDev, "parent", "1:",
		"protocol", "ip", "prio", prio, "u32",
		"match", "ip", "src", cidr,
		"flowid", cid).CombinedOutput()
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if strings.Contains(msg, "File exists") {
			return nil
		}
		return fmt.Errorf("ifb u32 cidr %s flowid %s: %s %w", cidr, cid, msg, err)
	}
	return nil
}

// FlushIFBUploadU32 删除 ifb0 egress 上全部上行 u32（replay 前去重）
func FlushIFBUploadU32() {
	for i := 0; i < 128; i++ {
		out, _ := exec.Command("tc", "filter", "del", "dev", IFBDev, "parent", "1:",
			"protocol", "ip", "u32").CombinedOutput()
		msg := string(out)
		if strings.Contains(msg, "No such file") || strings.Contains(msg, "Cannot find") {
			break
		}
	}
}

func removeIFBUploadFilterCIDR(cidr string) error {
	_ = exec.Command("tc", "filter", "del", "dev", IFBDev, "parent", "1:",
		"protocol", "ip", "u32", "match", "ip", "src", cidr).Run()
	return nil
}
