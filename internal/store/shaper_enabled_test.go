package store

import (
	"encoding/json"
	"testing"
)

func TestShaperImpliesEnabled(t *testing.T) {
	if ShaperImpliesEnabled(ShaperState{}) {
		t.Fatal("empty shaper should not imply enabled")
	}
	if !ShaperImpliesEnabled(ShaperState{Profiles: []ProfileEntry{{CIDR: "10.0.0.0/8"}}}) {
		t.Fatal("profiles should imply enabled")
	}
	if !ShaperImpliesEnabled(ShaperState{DefaultProfile: RateProfile{Down: "8mbit", Up: "8mbit"}}) {
		t.Fatal("default rate should imply enabled")
	}
}

func TestMigrateShaperEnabled(t *testing.T) {
	raw := []byte(`{"profiles":[{"cidr":"10.0.0.0/8","down":"8mbit","up":"8mbit"}]}`)
	var sh ShaperState
	if err := json.Unmarshal(raw, &sh); err != nil {
		t.Fatal(err)
	}
	if sh.Enabled {
		t.Fatal("should start false before migration")
	}
	MigrateShaperEnabled(raw, &sh)
	if !sh.Enabled {
		t.Fatal("expected auto-enable when profiles exist")
	}

	explicit := []byte(`{"enabled":false,"profiles":[{"cidr":"10.0.0.0/8"}]}`)
	sh = ShaperState{}
	_ = json.Unmarshal(explicit, &sh)
	MigrateShaperEnabled(explicit, &sh)
	if sh.Enabled {
		t.Fatal("explicit enabled:false must be preserved")
	}
}

func TestDefaultStateShaperDisabled(t *testing.T) {
	if DefaultState().Shaper.Enabled {
		t.Fatal("fresh install should default QoS off")
	}
}
