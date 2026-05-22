package store

import "testing"

func TestNormalizeTenant(t *testing.T) {
	tn := &TenantEntry{Name: "cdn", CIDRs: []string{"10.0.0.0/24", "10.0.1.0/24"}, Down: "50mbit", Up: "50mbit"}
	if err := NormalizeTenant(tn); err != nil {
		t.Fatal(err)
	}
	if len(tn.CIDRs) != 2 || tn.ID == "" {
		t.Fatalf("%+v", tn)
	}
}
