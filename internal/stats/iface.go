package stats

import (
	"bufio"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// QueueInfo 网卡多队列 / RSS
type QueueInfo struct {
	Device       string `json:"device"`
	Channels     int    `json:"channels"`
	RxQueues     int    `json:"rx_queues"`
	TxQueues     int    `json:"tx_queues"`
	Combined     int    `json:"combined"`
	IRQLines     []IRQStat `json:"irq_lines,omitempty"`
	Softnet      SoftnetStat `json:"softnet"`
}

// IRQStat 中断行（常对应 RSS 队列）
type IRQStat struct {
	IRQ    string `json:"irq"`
	CPU    string `json:"cpu_spread"`
	Count  uint64 `json:"count"`
}

// SoftnetStat 来自 /proc/net/softnet_stat 汇总
type SoftnetStat struct {
	Processed uint64 `json:"processed"`
	Dropped   uint64 `json:"dropped"`
	TimeSqueeze uint64 `json:"time_squeeze"`
}

// IfaceQueues 采集指定网卡队列与中断信息
func IfaceQueues(dev string) QueueInfo {
	q := QueueInfo{Device: dev}
	parseEthtoolChannels(dev, &q)
	q.IRQLines = parseInterrupts(dev)
	q.Softnet = parseSoftnet()
	return q
}

func parseEthtoolChannels(dev string, q *QueueInfo) {
	out, err := exec.Command("ethtool", "-l", dev).CombinedOutput()
	if err != nil {
		return
	}
	inCurrent := false
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Current") {
			inCurrent = true
			continue
		}
		if strings.HasPrefix(line, "Pre-set") {
			inCurrent = false
			continue
		}
		if !inCurrent {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		n, _ := strconv.Atoi(parts[len(parts)-1])
		switch parts[0] {
		case "RX:":
			q.RxQueues = n
		case "TX:":
			q.TxQueues = n
		case "Combined:":
			q.Combined = n
		}
	}
	if q.Combined > 0 {
		q.Channels = q.Combined
	} else if q.RxQueues > 0 {
		q.Channels = q.RxQueues
	}
}

func parseInterrupts(dev string) []IRQStat {
	b, err := os.ReadFile("/proc/interrupts")
	if err != nil {
		return nil
	}
	var out []IRQStat
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := sc.Text()
		if !strings.Contains(line, dev) {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		irq := fields[0]
		var sum uint64
		cpus := []string{}
		for i := 1; i < len(fields); i++ {
			if strings.Contains(fields[i], ":") {
				break
			}
			v, err := strconv.ParseUint(fields[i], 10, 64)
			if err != nil {
				break
			}
			sum += v
			cpus = append(cpus, fields[i])
		}
		name := fields[len(fields)-1]
		if name != dev && !strings.HasPrefix(name, dev+"-") && !strings.Contains(line, dev) {
			// name at end
		}
		_ = name
		out = append(out, IRQStat{
			IRQ:   strings.TrimSuffix(irq, ":"),
			Count: sum,
			CPU:   strings.Join(cpus, ","),
		})
	}
	return out
}

func parseSoftnet() SoftnetStat {
	b, err := os.ReadFile("/proc/net/softnet_stat")
	if err != nil {
		return SoftnetStat{}
	}
	var s SoftnetStat
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 3 {
			continue
		}
		p, _ := strconv.ParseUint(fields[0], 16, 64)
		d, _ := strconv.ParseUint(fields[1], 16, 64)
		t, _ := strconv.ParseUint(fields[2], 16, 64)
		s.Processed += p
		s.Dropped += d
		s.TimeSqueeze += t
	}
	return s
}

// SoftirqNET 读取 /proc/softirqs NET 行
func SoftirqNET() map[string][]uint64 {
	b, err := os.ReadFile("/proc/softirqs")
	if err != nil {
		return nil
	}
	out := map[string][]uint64{}
	for _, line := range strings.Split(string(b), "\n") {
		if !strings.HasPrefix(line, "NET_") {
			continue
		}
		parts := strings.Fields(line)
		name := strings.TrimSuffix(parts[0], ":")
		var vals []uint64
		for _, p := range parts[1:] {
			v, _ := strconv.ParseUint(p, 10, 64)
			vals = append(vals, v)
		}
		out[name] = vals
	}
	return out
}
