package ebpf

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"sync"

	"github.com/cilium/ebpf/ringbuf"
)

const ringbufWorkers = 4

type ringbufJob struct {
	ip    string
	down  uint64
	up    uint64
	minor uint32
}

// HostEnsurer 收到 NEW_HOST 时建 HTB 类
type HostEnsurer interface {
	EnsureHost(ip string, downBPS, upBPS uint64, minor uint32) error
}

// StartRingbuf 读取 events ringbuf（多 worker 并行 EnsureHost，加速 /24 等网段首包建类）
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
	jobs := make(chan ringbufJob, 256)
	var wg sync.WaitGroup
	for i := 0; i < ringbufWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if hs == nil {
					continue
				}
				if err := hs.EnsureHost(j.ip, j.down, j.up, j.minor); err != nil {
					log.Printf("ringbuf ensure %s: %v", j.ip, err)
				}
			}
		}()
	}
	go func() {
		defer rd.Close()
		defer close(jobs)
		defer wg.Wait()
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
			select {
			case jobs <- ringbufJob{ip: ip, down: down, up: up, minor: minor}:
			case <-ctx.Done():
				return
			}
		}
	}()
	return nil
}
