package shaper

import (
	"log"
	"time"

	"github.com/hk59775634/qosnat2/internal/ebpf"
)

// StaleEntry GC 候选
type StaleEntry struct {
	IP         string
	ClassMinor uint32
	LastSeenNS uint64
}

// GCRunner 空闲超时回收 HTB 类与 active_host
type GCRunner struct {
	Hosts    *HostShaper
	BPF      *ebpf.Manager
	Timeout  time.Duration
	KeepVIP  func() map[string]bool
}

// RunOnce 执行一轮 GC
func (g *GCRunner) RunOnce() (int, error) {
	if g.BPF == nil || !g.BPF.Ready() || g.Hosts == nil {
		return 0, nil
	}
	active, err := g.BPF.ListActive()
	if err != nil {
		return 0, err
	}
	now := uint64(time.Now().UnixNano())
	threshold := uint64(g.Timeout.Nanoseconds())
	var n int
	keep := map[string]bool{}
	if g.KeepVIP != nil {
		keep = g.KeepVIP()
	}
	for _, a := range active {
		if keep[a.IP] {
			continue
		}
		if a.LastSeenNS == 0 || now-a.LastSeenNS < threshold {
			continue
		}
		if err := g.Hosts.DeleteHost(a.IP); err != nil {
			log.Printf("gc htb %s: %v", a.IP, err)
		}
		if err := g.BPF.PurgeActive(a.IP); err != nil {
			log.Printf("gc active %s: %v", a.IP, err)
		}
		n++
	}
	if n > 0 {
		log.Printf("shaper gc: removed %d idle hosts (timeout %s)", n, g.Timeout)
	}
	return n, nil
}

// StartLoop 后台周期 GC
func StartLoop(stop <-chan struct{}, interval time.Duration, g *GCRunner) {
	if interval <= 0 {
		interval = time.Minute
	}
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			_, _ = g.RunOnce()
		}
	}
}
