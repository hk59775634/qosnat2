package store

import "testing"

func TestMigrateLegacyShaperMode(t *testing.T) {
	sh := ShaperState{Mode: "htb"}
	MigrateLegacyShaperMode(&sh)
	if sh.Mode != "" {
		t.Fatalf("expected empty mode, got %q", sh.Mode)
	}
	sh.Mode = "edt"
	MigrateLegacyShaperMode(&sh)
	if sh.Mode != "" {
		t.Fatalf("expected empty mode for edt, got %q", sh.Mode)
	}
}
