package route

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestRouteAlreadyAppliedSingle(t *testing.T) {
	idx := liveIndex{byKey: map[string]LiveRoute{
		store.RouteKey("default", "10.0.0.1", "eth0", 254): {
			Dest: "default", Gateway: "10.0.0.1", Device: "eth0", Metric: 100, Table: 254,
		},
	}}
	r := store.RouteEntry{Dest: "default", Gateway: "10.0.0.1", Device: "eth0", Metric: 100, Enabled: true}
	if !routeAlreadyApplied(r, idx) {
		t.Fatal("expected match")
	}
	r.Metric = 200
	if routeAlreadyApplied(r, idx) {
		t.Fatal("metric mismatch should not skip")
	}
}

func TestNeedsInfer(t *testing.T) {
	if needsInfer(store.RouteEntry{Gateway: "10.0.0.1", Device: "eth0"}) {
		t.Fatal("device set")
	}
	if !needsInfer(store.RouteEntry{Gateway: "10.0.0.1"}) {
		t.Fatal("missing device with gateway")
	}
}
