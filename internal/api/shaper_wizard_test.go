package api

import (
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestCaptureShaperWizardBackup(t *testing.T) {
	st := store.State{}
	st.Shaper.Profiles = []store.ProfileEntry{{CIDR: "10.0.0.0/8", Down: "8mbit"}}
	st.Shaper.PolicyCIDR = "192.168.1.0/24"
	st.Shaper.DefaultProfile = store.RateProfile{Down: "1mbit", Up: "1mbit"}
	st.Nat.IPv4.PolicyRoutes = []string{"10.0.0.0/8"}

	b := captureShaperWizardBackup(st)
	if len(b.profiles) != 1 || b.profiles[0].CIDR != "10.0.0.0/8" {
		t.Fatalf("profiles backup: %+v", b.profiles)
	}
	if b.policyCIDR != "192.168.1.0/24" {
		t.Fatalf("policy cidr: %q", b.policyCIDR)
	}
	if len(b.policyRoutes) != 1 {
		t.Fatalf("policy routes: %+v", b.policyRoutes)
	}

	st.Shaper.Profiles = nil
	if len(b.profiles) != 1 {
		t.Fatal("backup should be independent copy")
	}
}
