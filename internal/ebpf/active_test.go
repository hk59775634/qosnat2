package ebpf

import (
	"testing"
	"time"
)

func TestRateFromSamples(t *testing.T) {
	prevAt := time.Now().Add(-2 * time.Second)
	now := time.Now()
	// 2s 内下行 1.25MB → 625000 B/s = 5 Mbit/s；上行 0.625MB → 2.5 Mbit/s
	prev := map[uint32]activeSample{1: {down: 1000, up: 500}}
	cur := map[uint32]activeSample{1: {down: 1000 + 1_250_000, up: 500 + 625_000}}
	rates := rateFromSamples(prev, cur, prevAt, now)
	r := rates[1]
	if r.down < 600_000 || r.down > 650_000 {
		t.Fatalf("down B/s %d want ~625000", r.down)
	}
	if r.up < 300_000 || r.up > 325_000 {
		t.Fatalf("up B/s %d want ~312500", r.up)
	}
	if len(rateFromSamples(nil, cur, prevAt, now)) != 0 {
		t.Fatal("no prev should yield empty rates")
	}
}
