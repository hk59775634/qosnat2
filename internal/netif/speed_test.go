package netif

import "testing"

func TestParseEthtoolSpeed(t *testing.T) {
	sample := `Settings for eth0:
	Speed: 1000Mb/s
	Duplex: Full
`
	if got := parseEthtoolSpeed(sample); got != 1000 {
		t.Fatalf("1000Mb/s: got %d", got)
	}
	if got := parseEthtoolSpeed("Speed: 10000Mb/s\n"); got != 10000 {
		t.Fatalf("10000Mb/s: got %d", got)
	}
	if got := parseEthtoolSpeed("\tSpeed: 10Gb/s\n"); got != 10000 {
		t.Fatalf("10Gb/s: got %d", got)
	}
	if got := parseEthtoolSpeed("Speed: Unknown!\n"); got != 0 {
		t.Fatalf("unknown: got %d", got)
	}
	if got := parseEthtoolSpeed("Speed: 100Mb/s\n"); got != 100 {
		t.Fatalf("100Mb/s: got %d", got)
	}
}
