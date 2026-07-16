package api

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/hk59775634/qosnat2/internal/frr"
	"github.com/hk59775634/qosnat2/internal/route"
	"github.com/hk59775634/qosnat2/internal/store"
)

const (
	routeGuardInterval = 30 * time.Second
	routeGuardSettle   = 8 * time.Second
	routeGuardMinGap   = 45 * time.Second
)

func (srv *Server) startRouteGuardBackground(ctx context.Context) {
	var (
		mu         sync.Mutex
		lastReplay time.Time
	)
	go func() {
		ticker := time.NewTicker(routeGuardInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.routeGuardTick(ctx, &mu, &lastReplay)
			}
		}
	}()
}

func (srv *Server) routeGuardTick(ctx context.Context, mu *sync.Mutex, lastReplay *time.Time) {
	if !srv.setupComplete() {
		return
	}
	st := srv.store.Get()
	if len(st.Routes) == 0 {
		return
	}
	missing, err := route.MissingManaged(st.Routes)
	if err != nil {
		log.Printf("route guard: list: %v", err)
		return
	}
	if len(missing) == 0 {
		return
	}
	select {
	case <-ctx.Done():
		return
	case <-time.After(routeGuardSettle):
	}
	missing, err = route.MissingManaged(srv.store.Get().Routes)
	if err != nil {
		log.Printf("route guard: recheck: %v", err)
		return
	}
	if len(missing) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if time.Since(*lastReplay) < routeGuardMinGap {
		return
	}
	st = srv.store.Get()
	if route.NormalizeBackend(st.System.RouteBackend) == route.BackendFRR {
		if !frr.WaitActive(20 * time.Second) {
			log.Printf("route guard: frr not active, skip replay (%s)", formatMissingRoutes(missing))
			return
		}
	}
	res, err := route.ApplyFromState(st)
	if err != nil {
		log.Printf("route guard: replay failed (%s): %v", formatMissingRoutes(missing), err)
		return
	}
	*lastReplay = time.Now()
	log.Printf("route guard: replayed missing routes (%s) backend=%s applied=%d",
		formatMissingRoutes(missing), res.Backend, res.Applied)
}

func formatMissingRoutes(routes []store.RouteEntry) string {
	parts := make([]string, 0, len(routes))
	for _, r := range routes {
		parts = append(parts, r.Dest)
	}
	return strings.Join(parts, ", ")
}
