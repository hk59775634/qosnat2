package api

import (
	"context"
	"log"
	"strings"
	"time"

	"github.com/hk59775634/qosnat2/internal/store"
)

func (srv *Server) startScheduleBackground(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		var lastSig string
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sig := scheduleActiveSignature(srv.store.Get())
				if sig == lastSig {
					continue
				}
				if lastSig != "" {
					if err := srv.reloadNft(); err != nil {
						log.Printf("schedule reload nft: %v", err)
					} else {
						log.Printf("firewall schedules window changed; nft reloaded")
					}
				}
				lastSig = sig
			}
		}
	}()
}

func scheduleActiveSignature(st store.State) string {
	now := time.Now()
	var b strings.Builder
	for _, r := range st.Firewall.FilterRules {
		if strings.TrimSpace(r.ScheduleID) == "" {
			continue
		}
		on := store.RuleEffectivelyEnabled(r, st.Firewall.Schedules, now)
		b.WriteString(r.ID)
		if on {
			b.WriteByte('1')
		} else {
			b.WriteByte('0')
		}
		b.WriteByte('|')
	}
	return b.String()
}
