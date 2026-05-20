package store

import "testing"

func TestReorderProfiles(t *testing.T) {
	in := []ProfileEntry{
		{CIDR: "10.0.0.0/8", Down: "8mbit", Up: "8mbit", ID: 1},
		{CIDR: "10.0.1.0/24", Down: "50mbit", Up: "50mbit", ID: 2},
	}
	out, err := ReorderProfiles(in, []string{"10.0.1.0/24", "10.0.0.0/8"})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 || out[0].CIDR != "10.0.1.0/24" || out[0].ID != 1 {
		t.Fatalf("unexpected: %+v", out)
	}
	if out[1].ID != 2 {
		t.Fatalf("second id: %d", out[1].ID)
	}
}

func TestMigrateProfilePriorityToID(t *testing.T) {
	profiles := []ProfileEntry{
		{CIDR: "10.0.0.0/8", Priority: 10},
		{CIDR: "10.0.1.0/24", ID: 5},
	}
	MigrateProfilePriorityToID(&profiles)
	if profiles[0].ID != 10 {
		t.Fatalf("expected id 10, got %d", profiles[0].ID)
	}
	if profiles[1].ID != 5 {
		t.Fatalf("expected id 5, got %d", profiles[1].ID)
	}
}

func TestSortProfilesByID(t *testing.T) {
	in := []ProfileEntry{
		{CIDR: "10.0.0.0/8", ID: 20},
		{CIDR: "10.0.1.0/24", ID: 5},
	}
	out := SortProfilesByID(in)
	if out[0].ID != 5 || out[1].ID != 20 {
		t.Fatalf("sort order: %+v", out)
	}
}
