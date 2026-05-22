package netif

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// LinkSpeedMbps 读取网卡协商速率（Mbps）；未知或虚拟口返回 0
func LinkSpeedMbps(dev string) int {
	if dev == "" {
		return 0
	}
	if v := linkSpeedFromSysfs(dev); v > 0 {
		return v
	}
	return linkSpeedFromEthtool(dev)
}

func linkSpeedFromSysfs(dev string) int {
	b, err := os.ReadFile("/sys/class/net/" + dev + "/speed")
	if err != nil {
		return 0
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || v <= 0 || v == 65535 {
		return 0
	}
	return v
}

func linkSpeedFromEthtool(dev string) int {
	out, err := exec.Command("ethtool", dev).CombinedOutput()
	if err != nil {
		return 0
	}
	return parseEthtoolSpeed(string(out))
}

// parseEthtoolSpeed 解析 ethtool 输出中的 Speed 行（如 1000Mb/s、10Gb/s）
func parseEthtoolSpeed(s string) int {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		i := strings.Index(line, "Speed:")
		if i < 0 {
			continue
		}
		val := strings.TrimSpace(line[i+len("Speed:"):])
		if v := parseSpeedToken(val); v > 0 {
			return v
		}
	}
	return 0
}

func parseSpeedToken(tok string) int {
	tok = strings.TrimSpace(strings.TrimSuffix(tok, "!"))
	if tok == "" {
		return 0
	}
	low := strings.ToLower(tok)
	if strings.Contains(low, "unknown") {
		return 0
	}
	i := 0
	for i < len(low) && low[i] >= '0' && low[i] <= '9' {
		i++
	}
	if i == 0 {
		return 0
	}
	n, err := strconv.Atoi(low[:i])
	if err != nil || n <= 0 {
		return 0
	}
	rest := low[i:]
	switch {
	case strings.HasPrefix(rest, "gb"):
		return n * 1000
	case strings.HasPrefix(rest, "mb"):
		return n
	default:
		return 0
	}
}
