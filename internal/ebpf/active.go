package ebpf

import (
	"encoding/binary"
	"errors"
)

const (
	dirDown byte = 0
	dirUp   byte = 1
)

// ActiveEntry 近期有流量的 Per-IP 限速状态（来自 throttle / token_bucket LRU）。
type ActiveEntry struct {
	IP        string `json:"ip"`
	BytesDown uint64 `json:"bytes_down"`
	BytesUp   uint64 `json:"bytes_up"`
}

// ListActive 枚举运行期活跃主机；Bytes* 为 bps 量级代理值，供仪表盘排序。
func (m *Manager) ListActive() ([]ActiveEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return nil, errors.New("ebpf not loaded")
	}
	byIP := map[uint32]*ActiveEntry{}
	addScore := func(ip uint32, down, up uint64) {
		if ip == 0 {
			return
		}
		e, ok := byIP[ip]
		if !ok {
			e = &ActiveEntry{IP: HostKeyToIP(ip)}
			byIP[ip] = e
		}
		e.BytesDown += down
		e.BytesUp += up
	}
	iter := m.objs.Throttle.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 5 || len(vbuf) < 24 {
			continue
		}
		ip := binary.BigEndian.Uint32(kbuf[0:4])
		dir := kbuf[4]
		bps := binary.LittleEndian.Uint64(vbuf[8:16])
		switch dir {
		case dirDown:
			addScore(ip, bps, 0)
		case dirUp:
			addScore(ip, 0, bps)
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	iter = m.objs.TokenBucket.Iterate()
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 4 || len(vbuf) < 24 {
			continue
		}
		ip := binary.BigEndian.Uint32(kbuf[0:4])
		bps := binary.LittleEndian.Uint64(vbuf[16:24])
		addScore(ip, bps/2, bps/2)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	out := make([]ActiveEntry, 0, len(byIP))
	for _, e := range byIP {
		out = append(out, *e)
	}
	return out, nil
}
