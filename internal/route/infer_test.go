package route

import (
	"strings"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestInferRouteDevicesPreservesExplicitDevice(t *testing.T) {
	r := store.RouteEntry{
		Gateway: "10.0.0.1",
		Device:  "eth0",
	}
	out := InferRouteDevices(r)
	if out.Device != "eth0" {
		t.Fatalf("device=%q", out.Device)
	}
}

func TestInferRouteDevicesGatewayOnlySkipsLo(t *testing.T) {
	r := InferRouteDevices(store.RouteEntry{
		Dest:    "203.0.113.5/32",
		Gateway: "127.0.0.1",
	})
	if r.Device == "lo" {
		t.Fatalf("must not infer lo as egress device, got %q", r.Device)
	}
}

func TestBuildReplaceArgsIncludesInferredDevice(t *testing.T) {
	r := InferRouteDevices(store.RouteEntry{
		Dest:    "203.0.113.5/32",
		Gateway: "127.0.0.1",
	})
	args, err := buildReplaceArgs(r)
	if err != nil {
		t.Fatal(err)
	}
	if r.Device == "" {
		t.Skip("no device inferred in this environment")
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "dev "+r.Device) {
		t.Fatalf("args missing dev: %s", joined)
	}
}
