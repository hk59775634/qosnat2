package api

import (
	"context"
	"log"
	"time"
)

// aliasRefreshInterval FQDN/URL 别名后台刷新周期。
const aliasRefreshInterval = 5 * time.Minute

func (srv *Server) startAliasRefreshBackground(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(aliasRefreshInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.setupComplete() {
					continue
				}
				if err := srv.withNftApply(func() error {
					if warns := srv.refreshDynamicAliasesLocked(); len(warns) > 0 {
						for _, w := range warns {
							log.Printf("alias refresh: %s", w)
						}
					}
					return srv.applyNftLocked()
				}); err != nil {
					log.Printf("alias refresh apply: %v", err)
				}
			}
		}
	}()
}
