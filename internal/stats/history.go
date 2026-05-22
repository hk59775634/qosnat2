package stats

import "time"

const maxHistoryPoints = 2880 // 约 4 小时 @ 5s

// TrafficPoint 单点采样
type TrafficPoint struct {
	Ts     int64   `json:"ts"`
	LanRx  float64 `json:"lan_rx_mbps"`
	LanTx  float64 `json:"lan_tx_mbps"`
	WanRx  float64 `json:"wan_rx_mbps"`
	WanTx  float64 `json:"wan_tx_mbps"`
	CPU    float64 `json:"cpu_percent"`
	Connt  int     `json:"conntrack"`
}

// RecordTraffic 记录 LAN/WAN 吞吐与系统指标（由后台采样调用）
func (c *Collector) RecordTraffic(devLAN, devWAN string) {
	lan := c.IfaceMbps(devLAN)
	wan := c.IfaceMbps(devWAN)
	sys := c.System()
	pt := TrafficPoint{
		Ts:    time.Now().Unix(),
		LanRx: lan.RxMbps,
		LanTx: lan.TxMbps,
		WanRx: wan.RxMbps,
		WanTx: wan.TxMbps,
		CPU:   sys.CPUPercent,
		Connt: sys.Conntrack,
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.history = append(c.history, pt)
	if len(c.history) > maxHistoryPoints {
		c.history = c.history[len(c.history)-maxHistoryPoints:]
	}
}

// TrafficHistory 返回历史序列副本
func (c *Collector) TrafficHistory() []TrafficPoint {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.history) == 0 {
		return []TrafficPoint{}
	}
	out := make([]TrafficPoint, len(c.history))
	copy(out, c.history)
	return out
}

// InitHistoryFields 确保 Collector 含 history 切片
func (c *Collector) initHistory() {
	c.mu.Lock()
	if c.history == nil {
		c.history = []TrafficPoint{}
	}
	c.mu.Unlock()
}
