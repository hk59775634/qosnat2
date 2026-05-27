package wg

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// PeerTransferStats 来自 `wg show IFACE dump` 单行 peer 统计
type PeerTransferStats struct {
	PublicKey     string
	RxBytes       uint64
	TxBytes       uint64
	LastHandshake time.Time // 零值表示尚未握手
}

// ParseShowDumpOutput 解析 `wg show <iface> dump` 的标准输出（首行为设备行，其后为 peer 行）。
func ParseShowDumpOutput(output string) ([]PeerTransferStats, error) {
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) == 0 {
		return nil, nil
	}
	var out []PeerTransferStats
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, "\t")
		// 设备行：private_key public_key listen_port fwmark
		if len(parts) == 4 {
			continue
		}
		if len(parts) < 8 {
			continue
		}
		pub := strings.TrimSpace(parts[0])
		if pub == "" {
			continue
		}
		sec, err1 := strconv.ParseInt(strings.TrimSpace(parts[4]), 10, 64)
		var nsec int64
		var rx, tx uint64
		var err2, err3, err4 error
		if len(parts) >= 9 {
			// 兼容 sec+nsec 格式
			nsec, err2 = strconv.ParseInt(strings.TrimSpace(parts[5]), 10, 64)
			rx, err3 = strconv.ParseUint(strings.TrimSpace(parts[6]), 10, 64)
			tx, err4 = strconv.ParseUint(strings.TrimSpace(parts[7]), 10, 64)
		} else {
			// `wg show IFACE dump` 常见格式：sec, rx, tx（无 nsec）
			err2 = nil
			rx, err3 = strconv.ParseUint(strings.TrimSpace(parts[5]), 10, 64)
			tx, err4 = strconv.ParseUint(strings.TrimSpace(parts[6]), 10, 64)
		}
		if err1 != nil || err2 != nil || err3 != nil || err4 != nil {
			continue
		}
		var hs time.Time
		if sec != 0 || nsec != 0 {
			hs = time.Unix(sec, nsec)
		}
		out = append(out, PeerTransferStats{
			PublicKey:     pub,
			RxBytes:       rx,
			TxBytes:       tx,
			LastHandshake: hs,
		})
	}
	return out, nil
}

// DumpPeerStats 执行 `wg show <iface> dump` 并解析为按公钥索引的 map。
func DumpPeerStats(iface string) (map[string]PeerTransferStats, error) {
	if iface == "" {
		iface = "wg0"
	}
	out, err := exec.Command("wg", "show", iface, "dump").CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("wg show %s dump: %w: %s", iface, err, strings.TrimSpace(string(out)))
	}
	rows, err := ParseShowDumpOutput(string(out))
	if err != nil {
		return nil, err
	}
	m := make(map[string]PeerTransferStats, len(rows))
	for _, r := range rows {
		m[r.PublicKey] = r
	}
	return m, nil
}

// PeerLikelyOnline 根据最近一次握手时间判断是否可能在线（握手后窗口内视为在线）。
func PeerLikelyOnline(lastHandshake time.Time, now time.Time, window time.Duration) bool {
	if lastHandshake.IsZero() {
		return false
	}
	return now.Sub(lastHandshake) <= window
}
