package tuning

import "testing"

func TestConntrackBudget16GB(t *testing.T) {
	b := ConntrackBudget(HostInfo{MemMB: 16384})
	if b.OptimizationMB != 8192 {
		t.Fatalf("opt mb: got %d want 8192", b.OptimizationMB)
	}
	if b.ConntrackMax != 16777216 {
		t.Fatalf("max: got %d want 16777216", b.ConntrackMax)
	}
	if b.ConntrackBuckets != 4194304 {
		t.Fatalf("buckets: got %d want 4194304", b.ConntrackBuckets)
	}
}

func TestConntrackBudget8GB(t *testing.T) {
	b := ConntrackBudget(HostInfo{MemMB: 8192})
	if b.ConntrackMax != 8388608 {
		t.Fatalf("max: got %d want 8388608", b.ConntrackMax)
	}
	if b.ConntrackBuckets != 2097152 {
		t.Fatalf("buckets: got %d want 2097152", b.ConntrackBuckets)
	}
}

func TestRoundUpPower2(t *testing.T) {
	if roundUpPower2(4194304) != 4194304 {
		t.Fatal("power2 identity")
	}
	if roundUpPower2(4194305) != 8388608 {
		t.Fatal("round up")
	}
}
