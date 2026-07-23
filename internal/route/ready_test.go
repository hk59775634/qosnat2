package route

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/netif"
	"github.com/hk59775634/qosnat2/internal/store"
)

func TestPartitionByDeviceReady_noDevice(t *testing.T) {
	routes := []store.RouteEntry{
		{ID: "1", Dest: "default", Gateway: "1.2.3.4", Enabled: true},
	}
	ready, deferred := PartitionByDeviceReady(routes)
	if len(ready) != 1 || len(deferred) != 0 {
		t.Fatalf("ready=%d deferred=%d", len(ready), len(deferred))
	}
}

func TestPartitionByDeviceReady_loopbackExists(t *testing.T) {
	if !netif.LinkExists("lo") {
		t.Skip("no lo interface")
	}
	routes := []store.RouteEntry{
		{ID: "1", Dest: "default", Device: "lo", Enabled: true},
	}
	ready, deferred := PartitionByDeviceReady(routes)
	if len(ready) != 1 || len(deferred) != 0 {
		t.Fatalf("ready=%d deferred=%d", len(ready), len(deferred))
	}
}

func TestPartitionByDeviceReady_missingDevice(t *testing.T) {
	routes := []store.RouteEntry{
		{ID: "1", Dest: "default", Device: "qwp0-nonexistent-test", Enabled: true},
	}
	ready, deferred := PartitionByDeviceReady(routes)
	if len(ready) != 0 || len(deferred) != 1 {
		t.Fatalf("ready=%d deferred=%d", len(ready), len(deferred))
	}
}

func TestDeferredRouteDevices(t *testing.T) {
	deferred := []store.RouteEntry{
		{ID: "1", Dest: "default", Device: "qwp0"},
		{ID: "2", Dest: "default", Device: "qpe0"},
		{ID: "3", Dest: "default", Device: "qwp0"},
	}
	devs := DeferredRouteDevices(deferred)
	if len(devs) != 2 {
		t.Fatalf("want 2 unique devices, got %v", devs)
	}
}
