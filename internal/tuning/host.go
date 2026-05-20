package tuning

import (
	"os"
	"strconv"
	"strings"
)

// HostInfo 宿主机容量（用于推荐调优档位）
type HostInfo struct {
	CPUs       int   `json:"cpus"`
	MemTotalKB int64 `json:"mem_total_kb"`
	MemMB      int64 `json:"mem_mb"`
}

// Tier 调优档位
type Tier string

const (
	TierLow    Tier = "low"
	TierMedium Tier = "medium"
	TierHigh   Tier = "high"
)

// DetectHost 读取 CPU 核数与内存
func DetectHost() HostInfo {
	cpus := 1
	if out, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		n := 0
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "processor\t:") {
				n++
			}
		}
		if n > 0 {
			cpus = n
		}
	}
	memKB := int64(0)
	if out, err := os.ReadFile("/proc/meminfo"); err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.HasPrefix(line, "MemTotal:") {
				fields := strings.Fields(line)
				if len(fields) >= 2 {
					memKB, _ = strconv.ParseInt(fields[1], 10, 64)
				}
				break
			}
		}
	}
	memMB := memKB / 1024
	if memMB < 1 {
		memMB = 512
	}
	return HostInfo{CPUs: cpus, MemTotalKB: memKB, MemMB: memMB}
}

// ClassifyTier 按 CPU/内存划分 low / medium / high
func ClassifyTier(h HostInfo) Tier {
	if h.CPUs <= 2 || h.MemMB < 2048 {
		return TierLow
	}
	if h.CPUs <= 4 || h.MemMB < 8192 {
		return TierMedium
	}
	return TierHigh
}
