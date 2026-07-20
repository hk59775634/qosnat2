package ebpf

import (
	"encoding/binary"
	"errors"
	"sort"
	"time"
)

const (
	dirDown byte = 0
	dirUp   byte = 1
)

// ActiveHost 共享桶下的成员主机（host_flow）。
type ActiveHost struct {
	IP          string `json:"ip"`
	BytesDown   uint64 `json:"bytes_down"`
	BytesUp     uint64 `json:"bytes_up"`
	RateDownBPS uint64 `json:"rate_down_bps"`
	RateUpBPS   uint64 `json:"rate_up_bps"`
}

// ActiveEntry 限速桶观测项：mask<32 时 key 为聚合网段地址，hosts 为成员。
type ActiveEntry struct {
	IP           string       `json:"ip"` // 桶键（与 key 相同，兼容旧字段）
	Key          string       `json:"key"`
	Shared       bool         `json:"shared"`
	HostMask     uint8        `json:"host_mask"`
	DownBPS      uint64       `json:"down_bps"` // 配置
	UpBPS        uint64       `json:"up_bps"`
	BytesDown    uint64       `json:"bytes_down"`
	BytesUp      uint64       `json:"bytes_up"`
	RateDownBPS  uint64       `json:"rate_down_bps"` // 实测
	RateUpBPS    uint64       `json:"rate_up_bps"`
	ActivityDown uint64       `json:"activity_down"` // 兼容：等同 RateDownBPS
	ActivityUp   uint64       `json:"activity_up"`
	Hosts        []ActiveHost `json:"hosts,omitempty"`
}

// ListActive 枚举运行期限速桶；轮询间隔内根据字节增量推算瞬时速率。
func (m *Manager) ListActive() ([]ActiveEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if !m.loaded || m.objs == nil {
		return nil, errors.New("ebpf not loaded")
	}

	type bucketAcc struct {
		key                    uint32
		bytesDown, bytesUp     uint64
		cfgBPSDown, cfgBPSUp   uint64
		hostMask               uint8
		hosts                  []ActiveHost
	}
	byKey := map[uint32]*bucketAcc{}

	ensure := func(key uint32) *bucketAcc {
		e, ok := byKey[key]
		if !ok {
			e = &bucketAcc{key: key, hostMask: 32}
			byKey[key] = e
		}
		return e
	}

	iter := m.objs.Throttle.Iterate()
	var kbuf, vbuf []byte
	for iter.Next(&kbuf, &vbuf) {
		if len(kbuf) < 5 || len(vbuf) < 24 {
			continue
		}
		ip := binary.BigEndian.Uint32(kbuf[0:4])
		dir := kbuf[4]
		if dir != dirDown {
			continue
		}
		e := ensure(ip)
		if len(vbuf) >= 32 {
			e.bytesDown = binary.LittleEndian.Uint64(vbuf[24:32])
		}
		e.cfgBPSDown = binary.LittleEndian.Uint64(vbuf[8:16])
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
		e := ensure(ip)
		e.cfgBPSUp = binary.LittleEndian.Uint64(vbuf[16:24])
		if len(vbuf) >= 32 {
			e.bytesUp = binary.LittleEndian.Uint64(vbuf[24:32])
		}
	}
	if err := iter.Err(); err != nil {
		return nil, err
	}

	hostSnap := map[uint32]activeSample{}
	if m.objs.HostFlow != nil {
		iter = m.objs.HostFlow.Iterate()
		for iter.Next(&kbuf, &vbuf) {
			if len(kbuf) < 4 || len(vbuf) < 32 {
				continue
			}
			flowIP := binary.BigEndian.Uint32(kbuf[0:4])
			bucketBE := binary.BigEndian.Uint32(vbuf[0:4])
			mask := vbuf[4]
			if mask == 0 {
				mask = 32
			}
			bytesDown := binary.LittleEndian.Uint64(vbuf[8:16])
			bytesUp := binary.LittleEndian.Uint64(vbuf[16:24])
			hostSnap[flowIP] = activeSample{down: bytesDown, up: bytesUp}

			e := ensure(bucketBE)
			e.hostMask = mask
			e.hosts = append(e.hosts, ActiveHost{
				IP:        HostKeyToIP(flowIP),
				BytesDown: bytesDown,
				BytesUp:   bytesUp,
			})
		}
		if err := iter.Err(); err != nil {
			return nil, err
		}
	}

	now := time.Now()
	bucketSnap := map[uint32]activeSample{}
	for k, e := range byKey {
		bucketSnap[k] = activeSample{down: e.bytesDown, up: e.bytesUp}
		if down, up, mask, ok := m.lookupRateMetaLocked(k); ok {
			e.cfgBPSDown = down
			e.cfgBPSUp = up
			if mask != 0 {
				e.hostMask = mask
			}
		}
	}

	m.activeSampleMu.Lock()
	prevBucket := m.activePrev
	prevBucketAt := m.activePrevAt
	prevHost := m.hostPrev
	prevHostAt := m.hostPrevAt
	m.activePrev = bucketSnap
	m.activePrevAt = now
	m.hostPrev = hostSnap
	m.hostPrevAt = now
	m.activeSampleMu.Unlock()

	bucketRates := rateFromSamples(prevBucket, bucketSnap, prevBucketAt, now)
	hostRates := rateFromSamples(prevHost, hostSnap, prevHostAt, now)

	out := make([]ActiveEntry, 0, len(byKey))
	for _, e := range byKey {
		mask := e.hostMask
		if mask == 0 {
			mask = 32
		}
		rd, ru := bucketRates[e.key].down, bucketRates[e.key].up
		hosts := e.hosts
		for i := range hosts {
			if k, err := IPToHostKey(hosts[i].IP); err == nil {
				hosts[i].RateDownBPS = hostRates[k].down
				hosts[i].RateUpBPS = hostRates[k].up
			}
		}
		sort.Slice(hosts, func(i, j int) bool {
			return hosts[i].RateDownBPS+hosts[i].RateUpBPS > hosts[j].RateDownBPS+hosts[j].RateUpBPS
		})
		keyIP := HostKeyToIP(e.key)
		entry := ActiveEntry{
			IP:           keyIP,
			Key:          keyIP,
			Shared:       mask < 32,
			HostMask:     mask,
			DownBPS:      e.cfgBPSDown,
			UpBPS:        e.cfgBPSUp,
			BytesDown:    e.bytesDown,
			BytesUp:      e.bytesUp,
			RateDownBPS:  rd,
			RateUpBPS:    ru,
			ActivityDown: rd,
			ActivityUp:   ru,
			Hosts:        hosts,
		}
		out = append(out, entry)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].RateDownBPS+out[i].RateUpBPS > out[j].RateDownBPS+out[j].RateUpBPS
	})
	return out, nil
}

func rateFromSamples(prev, cur map[uint32]activeSample, prevAt, now time.Time) map[uint32]activeSample {
	out := make(map[uint32]activeSample, len(cur))
	dt := now.Sub(prevAt).Seconds()
	if dt < 0.2 || prev == nil {
		return out
	}
	for k, c := range cur {
		p, ok := prev[k]
		if !ok {
			continue
		}
		var rd, ru uint64
		// 与 store.MbitToBPS / bpsLabel 一致：返回字节/秒，勿再 ×8
		if c.down >= p.down {
			rd = uint64(float64(c.down-p.down) / dt)
		}
		if c.up >= p.up {
			ru = uint64(float64(c.up-p.up) / dt)
		}
		out[k] = activeSample{down: rd, up: ru}
	}
	return out
}
