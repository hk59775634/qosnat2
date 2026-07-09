package ebpf

import (
	"encoding/binary"
	"errors"
)

const (
	dirDown byte = 0
	dirUp   byte = 1
)

// ActiveEntry 近期有流量的 Per-IP 限速状态（throttle / token_bucket LRU）。
type ActiveEntry struct {
	IP           string `json:"ip"`
	DownBPS      uint64 `json:"down_bps"`
	UpBPS        uint64 `json:"up_bps"`
	ActivityDown uint64 `json:"activity_down"`
	ActivityUp   uint64 `json:"activity_up"`
}

// ListActive 枚举运行期活跃主机；Activity* 为 bps 量级代理值供排序，DownBPS/UpBPS 来自 profile/host map。
func (m *Manager) ListActive() ([]ActiveEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return nil, errors.New("ebpf not loaded")
	}
	byIP := map[uint32]*ActiveEntry{}
	addActivity := func(ip uint32, downAct, upAct uint64) {
		if ip == 0 {
			return
		}
		e, ok := byIP[ip]
		if !ok {
			e = &ActiveEntry{IP: HostKeyToIP(ip)}
			byIP[ip] = e
		}
		e.ActivityDown += downAct
		e.ActivityUp += upAct
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
			addActivity(ip, bps, 0)
		case dirUp:
			addActivity(ip, 0, bps)
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
		// ingress token bucket 仅整形上行（源 IP）；勿计入下行活动
		addActivity(ip, 0, bps)
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}
	out := make([]ActiveEntry, 0, len(byIP))
	for _, e := range byIP {
		if k, err := IPToHostKey(e.IP); err == nil {
			if down, up, ok := m.lookupRatesLocked(k); ok {
				e.DownBPS = down
				e.UpBPS = up
			}
		}
		out = append(out, *e)
	}
	return out, nil
}
