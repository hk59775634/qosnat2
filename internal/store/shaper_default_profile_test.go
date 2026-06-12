package store

import "testing"

func TestMigrateStripDefaultProfileRates(t *testing.T) {
	sh := ShaperState{DefaultProfile: RateProfile{Down: "8mbit", Up: "8mbit", HostMask: 32}}
	if !MigrateStripDefaultProfileRates(&sh) {
		t.Fatal("expected change")
	}
	if sh.DefaultProfile.Down != "" || sh.DefaultProfile.Up != "" {
		t.Fatalf("rates not cleared: %+v", sh.DefaultProfile)
	}
	if sh.DefaultProfile.HostMask != 32 {
		t.Fatal("host_mask should be preserved")
	}
	if MigrateStripDefaultProfileRates(&sh) {
		t.Fatal("second call should be no-op")
	}
}
