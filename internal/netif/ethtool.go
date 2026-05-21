package netif

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// RingSettings 环缓冲与通道
type RingSettings struct {
	RxMax     int `json:"rx_max"`
	RxCurrent int `json:"rx_current"`
	TxMax     int `json:"tx_max"`
	TxCurrent int `json:"tx_current"`
}

// OffloadFlags 常见 offload 开关（能读则读）
type OffloadFlags struct {
	TCPRX     string `json:"tcp_rx,omitempty"`
	TCPTX     string `json:"tcp_tx,omitempty"`
	GRO       string `json:"gro,omitempty"`
	GSO       string `json:"gso,omitempty"`
	TXCSUM    string `json:"tx_checksum,omitempty"`
	RXCSUM    string `json:"rx_checksum,omitempty"`
}

// EthtoolInfo 网卡调优信息
type EthtoolInfo struct {
	Device   string       `json:"device"`
	Ring     RingSettings `json:"ring"`
	Offloads OffloadFlags `json:"offloads"`
}

// GetEthtool 读取 ethtool -g / -k
func GetEthtool(dev string) (EthtoolInfo, error) {
	if dev == "" {
		return EthtoolInfo{}, fmt.Errorf("empty device")
	}
	info := EthtoolInfo{Device: dev}
	out, err := exec.Command("ethtool", "-g", dev).CombinedOutput()
	if err == nil {
		info.Ring = parseRing(string(out))
	}
	out2, err2 := exec.Command("ethtool", "-k", dev).CombinedOutput()
	if err2 == nil {
		info.Offloads = parseOffloads(string(out2))
	}
	if err != nil && err2 != nil {
		return info, fmt.Errorf("ethtool: %s", strings.TrimSpace(string(out)))
	}
	return info, nil
}

// SetRing 设置 RX/TX ring（0 表示跳过该项）
func SetRing(dev string, rx, tx int) error {
	if dev == "" {
		return fmt.Errorf("empty device")
	}
	args := []string{"ethtool", "-G", dev}
	if rx > 0 {
		args = append(args, "rx", strconv.Itoa(rx))
	}
	if tx > 0 {
		args = append(args, "tx", strconv.Itoa(tx))
	}
	if len(args) <= 3 {
		return fmt.Errorf("rx or tx required")
	}
	out, err := exec.Command(args[0], args[1:]...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ethtool -G: %s %w", strings.TrimSpace(string(out)), err)
	}
	return nil
}

// OffloadSetRequest 写入 offload（on/off）
type OffloadSetRequest struct {
	GRO    string `json:"gro,omitempty"`
	GSO    string `json:"gso,omitempty"`
	TXCSUM string `json:"tx_checksum,omitempty"`
	RXCSUM string `json:"rx_checksum,omitempty"`
}

// SetOffloads 通过 ethtool -K 设置 offload
func SetOffloads(dev string, req OffloadSetRequest) error {
	if dev == "" {
		return fmt.Errorf("empty device")
	}
	type kv struct {
		flag string
		val  string
	}
	var items []kv
	if v := normalizeEthOnOff(req.GRO); v != "" {
		items = append(items, kv{"gro", v})
	}
	if v := normalizeEthOnOff(req.GSO); v != "" {
		items = append(items, kv{"gso", v})
	}
	if v := normalizeEthOnOff(req.TXCSUM); v != "" {
		items = append(items, kv{"tx", v})
	}
	if v := normalizeEthOnOff(req.RXCSUM); v != "" {
		items = append(items, kv{"rx", v})
	}
	if len(items) == 0 {
		return fmt.Errorf("no offload flags to set")
	}
	for _, it := range items {
		out, err := exec.Command("ethtool", "-K", dev, it.flag, it.val).CombinedOutput()
		if err != nil {
			return fmt.Errorf("ethtool -K %s %s: %s %w", it.flag, it.val, strings.TrimSpace(string(out)), err)
		}
	}
	return nil
}

func normalizeEthOnOff(v string) string {
	v = strings.ToLower(strings.TrimSpace(v))
	switch v {
	case "on", "off":
		return v
	default:
		return ""
	}
}

func parseRing(s string) RingSettings {
	var r RingSettings
	section := ""
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Pre-set maximums") {
			section = "max"
			continue
		}
		if strings.HasPrefix(line, "Current hardware settings") {
			section = "cur"
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		n, _ := strconv.Atoi(fields[1])
		switch fields[0] {
		case "RX:":
			if section == "max" {
				r.RxMax = n
			} else {
				r.RxCurrent = n
			}
		case "TX:":
			if section == "max" {
				r.TxMax = n
			} else {
				r.TxCurrent = n
			}
		}
	}
	return r
}

func parseOffloads(s string) OffloadFlags {
	var o OffloadFlags
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		val := parts[len(parts)-1]
		key := strings.Join(parts[:len(parts)-1], " ")
		switch {
		case strings.Contains(key, "tcp-segmentation-offload"):
			o.TCPTX = val
		case strings.Contains(key, "generic-receive-offload"):
			o.GRO = val
		case strings.Contains(key, "generic-segmentation-offload"):
			o.GSO = val
		case strings.Contains(key, "rx-checksumming"):
			o.RXCSUM = val
		case strings.Contains(key, "tx-checksumming"):
			o.TXCSUM = val
		}
	}
	return o
}
