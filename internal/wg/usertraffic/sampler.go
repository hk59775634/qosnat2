package usertraffic

import (
	"context"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
	"github.com/hk59775634/qosnat2/internal/wg"
)

// StartSampler 定期从各实例 `wg show IFACE dump` 采集 Peer transfer 计数
func StartSampler(ctx context.Context, getInstances func() []store.WireGuardInstance) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	poll := func() {
		for _, inst := range getInstances() {
			if !inst.Enabled {
				continue
			}
			iface := strings.TrimSpace(inst.Interface)
			if iface == "" {
				iface = "wg0"
			}
			stats, err := wg.DumpPeerStats(iface)
			if err != nil {
				continue
			}
			pubToName := map[string]string{}
			for _, p := range inst.Peers {
				pk := strings.TrimSpace(p.PublicKey)
				nm := strings.TrimSpace(p.Name)
				if pk != "" && nm != "" {
					pubToName[pk] = nm
				}
			}
			now := time.Now()
			s := DefaultStore()
			for pub, row := range stats {
				name := pubToName[pub]
				if name == "" {
					continue
				}
				key := store.WgPeerTrafficKey(inst.ID, name)
				_ = s.RecordCounters(key, row.RxBytes, row.TxBytes, now)
			}
		}
	}
	poll()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			poll()
		}
	}
}
