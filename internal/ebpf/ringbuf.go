package ebpf

import (
	"context"
	"encoding/binary"
	"errors"
	"log"

	"github.com/cilium/ebpf/ringbuf"
	"github.com/hk59775634/qosnat2/internal/store"
)

// HostEnsurer 收到 NEW_HOST 时建 HTB 类
type HostEnsurer interface {
	EnsureHost(ip string, downBPS, upBPS uint64, minor uint32) error
}

// StartRingbuf 读取 events ringbuf
func (m *Manager) StartRingbuf(ctx context.Context, hs HostEnsurer) error {
	m.mu.RLock()
	if !m.loaded || m.objs == nil {
		m.mu.RUnlock()
		return errors.New("ebpf not loaded")
	}
	rd, err := ringbuf.NewReader(m.objs.Events)
	m.mu.RUnlock()
	if err != nil {
		return err
	}
	go func() {
		defer rd.Close()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			rec, err := rd.Read()
			if err != nil {
				if errors.Is(err, ringbuf.ErrClosed) {
					return
				}
				if ctx.Err() != nil {
					return
				}
				log.Printf("ringbuf: %v", err)
				continue
			}
			if len(rec.RawSample) < 28 {
				continue
			}
			b := rec.RawSample
			/* BPF 写入 ip_host_key（与 IPToHostKey 相同，在 LE 上为 LittleEndian 存储） */
			ipBE := binary.LittleEndian.Uint32(b[0:4])
			down := binary.LittleEndian.Uint64(b[8:16])
			up := binary.LittleEndian.Uint64(b[16:24])
			minor := binary.LittleEndian.Uint32(b[24:28])
			ip := HostKeyToIP(ipBE)
			if hs != nil {
				if err := hs.EnsureHost(ip, down, up, minor); err != nil {
					log.Printf("ringbuf ensure %s: %v", ip, err)
				}
			}
		}
	}()
	return nil
}

// ReplayHostClasses 对已配置 host 预建 HTB
func ReplayHostClasses(st store.State, hs HostEnsurer) {
	if hs == nil {
		return
	}
	for _, p := range store.SortProfilesByID(st.Shaper.Profiles) {
		ip, ok := store.ProfileHostIP(p.CIDR)
		if !ok {
			continue
		}
		d, err := rateFromProfile(p.Down, p.Up)
		if err != nil {
			log.Printf("replay profile %s: %v", p.CIDR, err)
			continue
		}
		if err := hs.EnsureHost(ip, d.DownBPS, d.UpBPS, 0); err != nil {
			log.Printf("replay profile %s: %v", p.CIDR, err)
		}
	}
}
