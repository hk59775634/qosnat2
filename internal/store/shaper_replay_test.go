package store

import "testing"

func TestReplayPolicyCIDRToBPF(t *testing.T) {
	sh := ShaperState{
		PolicyCIDR:     "10.254.0.0/15",
		DefaultProfile: RateProfile{Down: "8mbit", Up: "8mbit"},
	}
	if ReplayPolicyCIDRToBPF(sh) {
		t.Fatal("empty profiles should not replay policy_cidr")
	}
	sh.Profiles = []ProfileEntry{{CIDR: "10.254.0.0/16", Down: "100mbit", Up: "100mbit"}}
	if !ReplayPolicyCIDRToBPF(sh) {
		t.Fatal("with profiles should replay policy_cidr default")
	}
	sh.DefaultProfile = RateProfile{}
	if ReplayPolicyCIDRToBPF(sh) {
		t.Fatal("unlimited default should not replay")
	}
}
