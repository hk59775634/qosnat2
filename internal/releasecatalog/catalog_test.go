package releasecatalog

import "testing"

func TestValidID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"2026052801", true},
		{"v2026052801", true},
		{"2026052899", true},
		{"2026052800", false},
		{"20260528100", false},
		{"2026132801", false},
		{"1.4.2", false},
	}
	for _, tc := range tests {
		if got := ValidID(tc.id); got != tc.want {
			t.Errorf("ValidID(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestGitHubTags(t *testing.T) {
	if got := QosnatGitHubTag("2026052803"); got != "v2026052803" {
		t.Fatalf("QosnatGitHubTag: %q", got)
	}
	if got := OcservGitHubTag("1.4.2"); got != "ocserv-1.4.2" {
		t.Fatalf("OcservGitHubTag: %q", got)
	}
}

func TestValidOcservVersion(t *testing.T) {
	for _, tc := range []struct {
		v    string
		want bool
	}{
		{"1.4.2", true},
		{"ocserv-1.4.2", true},
		{"1.3.0", true},
		{"2026052801", false},
		{"", false},
	} {
		if got := ValidOcservVersion(tc.v); got != tc.want {
			t.Errorf("ValidOcservVersion(%q) = %v, want %v", tc.v, got, tc.want)
		}
	}
}

func TestNormalizeOcservVersion(t *testing.T) {
	if got := NormalizeOcservVersion("ocserv-1.4.2"); got != "1.4.2" {
		t.Fatalf("NormalizeOcservVersion: %q", got)
	}
}
