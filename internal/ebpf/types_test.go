package ebpf

import "testing"

func TestNormalizeHostMask(t *testing.T) {
	cases := []struct {
		in   int
		want uint8
	}{
		{0, 32},
		{-1, 32},
		{33, 32},
		{1, 1},
		{24, 24},
		{30, 30},
		{32, 32},
	}
	for _, c := range cases {
		if got := NormalizeHostMask(c.in); got != c.want {
			t.Fatalf("NormalizeHostMask(%d)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestAggregateHostKey(t *testing.T) {
	host, err := IPToHostKey("10.0.0.5")
	if err != nil {
		t.Fatal(err)
	}
	if AggregateHostKey(host, 32) != host {
		t.Fatal("mask 32 should keep host key")
	}
	if AggregateHostKey(host, 0) != host {
		t.Fatal("mask 0 should keep host key")
	}
	agg30 := AggregateHostKey(host, 30)
	want30, _ := IPToHostKey("10.0.0.4")
	if agg30 != want30 {
		t.Fatalf("mask 30: got %s want %s", HostKeyToIP(agg30), HostKeyToIP(want30))
	}
	agg24 := AggregateHostKey(host, 24)
	want24, _ := IPToHostKey("10.0.0.0")
	if agg24 != want24 {
		t.Fatalf("mask 24: got %s want %s", HostKeyToIP(agg24), HostKeyToIP(want24))
	}
}

func TestRateValMarshalHostMask(t *testing.T) {
	b := RateVal{DownBPS: 1, UpBPS: 2, HostMask: 30}.Marshal()
	if len(b) != 24 {
		t.Fatalf("len=%d", len(b))
	}
	if b[20] != 30 {
		t.Fatalf("host_mask byte=%d", b[20])
	}
	b0 := RateVal{DownBPS: 1, UpBPS: 2}.Marshal()
	if b0[20] != 32 {
		t.Fatalf("zero HostMask should marshal as 32, got %d", b0[20])
	}
}
