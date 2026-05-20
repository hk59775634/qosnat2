package netif

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

const defaultTxQueueLen = 5000

// TxQueueLen 读取网卡 txqueuelen
func TxQueueLen(dev string) (int, error) {
	if dev == "" {
		return 0, fmt.Errorf("empty device")
	}
	out, err := exec.Command("ip", "-json", "link", "show", dev).Output()
	if err != nil {
		return 0, err
	}
	// 简单解析 "txqlen":5000
	s := string(out)
	i := strings.Index(s, `"txqlen":`)
	if i < 0 {
		return 0, nil
	}
	rest := s[i+len(`"txqlen":`):]
	end := strings.IndexAny(rest, ",}")
	if end < 0 {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil {
		return 0, err
	}
	return n, nil
}

// SetTxQueueLen 设置发送队列长度
func SetTxQueueLen(dev string, qlen int) error {
	if dev == "" {
		return fmt.Errorf("empty device")
	}
	if qlen <= 0 {
		qlen = defaultTxQueueLen
	}
	out, err := exec.Command("ip", "link", "set", dev, "txqueuelen", strconv.Itoa(qlen)).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ip link set %s txqueuelen %d: %s %w", dev, qlen, strings.TrimSpace(string(out)), err)
	}
	return nil
}

// ApplyRPS 为网卡 RX 队列配置 RPS（多核软中断分担）
func ApplyRPS(dev string) error {
	if dev == "" {
		return fmt.Errorf("empty device")
	}
	mask, err := rpsMask()
	if err != nil {
		return err
	}
	const rfs = 32768
	_ = os.WriteFile("/proc/sys/net/core/rps_sock_flow_entries", []byte(strconv.Itoa(rfs)), 0644)
	ncpu, _ := cpuCount()
	perQ := rfs
	if ncpu > 0 {
		perQ = rfs / ncpu
		if perQ < 1 {
			perQ = 1
		}
	}
	glob := filepath.Join("/sys/class/net", dev, "queues", "rx-*", "rps_cpus")
	matches, _ := filepath.Glob(glob)
	for _, p := range matches {
		_ = os.WriteFile(p, []byte(mask), 0644)
	}
	flowMatches, _ := filepath.Glob(filepath.Join("/sys/class/net", dev, "queues", "rx-*", "rps_flow_cnt"))
	for _, p := range flowMatches {
		_ = os.WriteFile(p, []byte(strconv.Itoa(perQ)), 0644)
	}
	return nil
}

func rpsMask() (string, error) {
	n, err := cpuCount()
	if err != nil || n < 1 {
		n = 1
	}
	var mask uint64
	for i := 0; i < n && i < 32; i++ {
		mask |= 1 << uint(i)
	}
	return fmt.Sprintf("%x", mask), nil
}

func cpuCount() (int, error) {
	out, err := exec.Command("nproc").Output()
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(strings.TrimSpace(string(out)))
}

// EffectiveTxQLen 0 表示使用默认 5000
func EffectiveTxQLen(configured int) int {
	if configured <= 0 {
		return defaultTxQueueLen
	}
	return configured
}
