package ebpf

import (
	"testing"
	"time"
)

func TestFilterActiveByIP(t *testing.T) {
	shared := ActiveEntry{
		IP:       "10.0.0.0",
		Key:      "10.0.0.0",
		Shared:   true,
		HostMask: 24,
		Hosts: []ActiveHost{
			{IP: "10.0.0.5"},
			{IP: "10.0.0.8"},
		},
	}
	solo := ActiveEntry{
		IP:       "10.1.0.9",
		Key:      "10.1.0.9",
		HostMask: 32,
	}
	list := []ActiveEntry{shared, solo}

	bucket24, err := IPToHostKey("10.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	got := filterActiveByIP(list, bucket24, "10.0.0.5")
	if len(got) != 1 || got[0].Key != "10.0.0.0" || len(got[0].Hosts) != 2 {
		t.Fatalf("shared bucket: %+v", got)
	}

	bucket32, err := IPToHostKey("10.1.0.9")
	if err != nil {
		t.Fatal(err)
	}
	got = filterActiveByIP(list, bucket32, "10.1.0.9")
	if len(got) != 1 || got[0].Key != "10.1.0.9" {
		t.Fatalf("solo bucket: %+v", got)
	}

	missing, err := IPToHostKey("10.2.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if got = filterActiveByIP(list, missing, "10.2.0.1"); len(got) != 0 {
		t.Fatalf("missing want empty, got %+v", got)
	}
}

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
