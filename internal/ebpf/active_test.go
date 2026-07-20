package ebpf

import (
	"testing"
	"time"
)

func TestRateFromSamples(t *testing.T) {
	prevAt := time.Now().Add(-2 * time.Second)
	now := time.Now()
	prev := map[uint32]activeSample{1: {down: 1000, up: 500}}
	cur := map[uint32]activeSample{1: {down: 1000 + 2_000_000, up: 500 + 1_000_000}}
	rates := rateFromSamples(prev, cur, prevAt, now)
	r := rates[1]
	// ~8 Mbps down, ~4 Mbps up over ~2s
	if r.down < 7_000_000 || r.down > 9_000_000 {
		t.Fatalf("down rate %d", r.down)
	}
	if r.up < 3_000_000 || r.up > 5_000_000 {
		t.Fatalf("up rate %d", r.up)
	}
	if len(rateFromSamples(nil, cur, prevAt, now)) != 0 {
		t.Fatal("no prev should yield empty rates")
	}
}
