package stats

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

// IfaceRates 接口吞吐（Mbps）
type IfaceRates struct {
	RxMbps float64 `json:"rx_mbps"`
	TxMbps float64 `json:"tx_mbps"`
}

// System 基础系统指标
type System struct {
	CPUPercent  float64 `json:"cpu_percent"`
	MemPercent  float64 `json:"mem_percent"`
	Conntrack   int     `json:"conntrack"`
	UptimeSec   float64 `json:"uptime_sec"`
}

// Collector 采样 /proc 与网卡计数
type Collector struct {
	mu       sync.Mutex
	last     time.Time
	prev     map[string][2]uint64 // dev -> rx, tx bytes
	prevCPU  [2]uint64
}

func New() *Collector { return &Collector{prev: map[string][2]uint64{}} }

func (c *Collector) System() System {
	return System{
		CPUPercent: c.cpuPercent(),
		MemPercent: memPercent(),
		Conntrack:  conntrackCount(),
		UptimeSec:  uptimeSec(),
	}
}

func (c *Collector) IfaceMbps(dev string) IfaceRates {
	if dev == "" {
		return IfaceRates{}
	}
	rx, tx := readIfaceBytes(dev)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prev == nil {
		c.prev = map[string][2]uint64{}
	}
	now := time.Now()
	dt := now.Sub(c.last).Seconds()
	prev, ok := c.prev[dev]
	if dt < 0.2 || c.last.IsZero() || !ok {
		c.last = now
		c.prev[dev] = [2]uint64{rx, tx}
		return IfaceRates{}
	}
	c.prev[dev] = [2]uint64{rx, tx}
	c.last = now
	rxMbps := float64(rx-prev[0]) * 8 / dt / 1e6
	txMbps := float64(tx-prev[1]) * 8 / dt / 1e6
	if rxMbps < 0 {
		rxMbps = 0
	}
	if txMbps < 0 {
		txMbps = 0
	}
	return IfaceRates{RxMbps: rxMbps, TxMbps: txMbps}
}

func readIfaceBytes(dev string) (rx, tx uint64) {
	read := func(suffix string) uint64 {
		b, err := os.ReadFile("/sys/class/net/" + dev + "/statistics/" + suffix)
		if err != nil {
			return 0
		}
		v, _ := strconv.ParseUint(strings.TrimSpace(string(b)), 10, 64)
		return v
	}
	return read("rx_bytes"), read("tx_bytes")
}

func (c *Collector) cpuPercent() float64 {
	b, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0
	}
	line := strings.TrimSpace(strings.Split(string(b), "\n")[0])
	fields := strings.Fields(line)
	if len(fields) < 8 {
		return 0
	}
	var total, idle uint64
	for i := 1; i < len(fields); i++ {
		v, _ := strconv.ParseUint(fields[i], 10, 64)
		total += v
		if i == 4 {
			idle = v
		}
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prevCPU[0] == 0 {
		c.prevCPU = [2]uint64{total, idle}
		return 0
	}
	dt := total - c.prevCPU[0]
	di := idle - c.prevCPU[1]
	c.prevCPU = [2]uint64{total, idle}
	if dt == 0 {
		return 0
	}
	return float64(dt-di) / float64(dt) * 100
}

func memPercent() float64 {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}
	vals := map[string]uint64{}
	s := bufio.NewScanner(strings.NewReader(string(b)))
	for s.Scan() {
		p := strings.Fields(s.Text())
		if len(p) < 2 {
			continue
		}
		v, _ := strconv.ParseUint(p[1], 10, 64)
		vals[p[0]] = v
	}
	total := vals["MemTotal:"]
	avail := vals["MemAvailable:"]
	if total == 0 {
		return 0
	}
	used := total - avail
	return float64(used) / float64(total) * 100
}

func conntrackCount() int {
	b, err := os.ReadFile("/proc/sys/net/netfilter/nf_conntrack_count")
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return n
}

func uptimeSec() float64 {
	b, err := os.ReadFile("/proc/uptime")
	if err != nil {
		return 0
	}
	f := strings.Fields(string(b))
	if len(f) == 0 {
		return 0
	}
	v, _ := strconv.ParseFloat(f[0], 64)
	return v
}
