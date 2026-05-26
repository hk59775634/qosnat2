package usertraffic

import "testing"

func TestFieldUintOcctlFormats(t *testing.T) {
	m := map[string]any{
		"RX":     "1376665000",
		"TX":     "331558000",
		"_RX":    "1.4 GB",
		"raw_rx": float64(1376665000),
	}
	if got := fieldUint(m, "raw_rx", "RX"); got != 1376665000 {
		t.Fatalf("raw_rx/RX: got %d", got)
	}
	if got := fieldUint(m, "_RX"); got != 0 {
		t.Fatalf("_RX should not parse: got %d", got)
	}
	if got := fieldUint(map[string]any{"RX": "42"}, "RX"); got != 42 {
		t.Fatalf("numeric string RX: got %d", got)
	}
}
