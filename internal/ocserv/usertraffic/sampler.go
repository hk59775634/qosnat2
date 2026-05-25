package usertraffic

import (
	"context"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/ocserv"
	"github.com/hk59775634/qosnat2/internal/store"
)

// StartSampler 定期从 occtl 采集在线用户流量增量
func StartSampler(ctx context.Context, getState func() store.OCServState) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	poll := func() {
		st := getState()
		if !st.Enabled || !st.Advanced.UseOcctl {
			return
		}
		cfg := ocserv.OcctlFromState(st)
		users, err := cfg.ShowUsers()
		if err != nil {
			return
		}
		now := time.Now()
		s := DefaultStore()
		for _, u := range users {
			name := fieldString(u, "Username", "username", "User", "user")
			if name == "" {
				continue
			}
			rx := fieldUint(u, "RX", "rx", "Rx")
			tx := fieldUint(u, "TX", "tx", "Tx")
			_ = s.RecordCounters(name, rx, tx, now)
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

func fieldString(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

func fieldUint(m map[string]any, keys ...string) uint64 {
	for _, k := range keys {
		v, ok := m[k]
		if !ok {
			continue
		}
		switch n := v.(type) {
		case float64:
			if n > 0 {
				return uint64(n)
			}
		case int:
			if n > 0 {
				return uint64(n)
			}
		case int64:
			if n > 0 {
				return uint64(n)
			}
		case uint64:
			return n
		case string:
			// 忽略已格式化的 _RX/_TX
		}
	}
	return 0
}
