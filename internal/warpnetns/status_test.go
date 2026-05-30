package warpnetns

import "testing"

func TestWarpStatusConnected(t *testing.T) {
	cases := []struct {
		raw  string
		want bool
	}{
		{"Status update: Connected\n", true},
		{"Status update: Disconnected\n", false},
		{"Unable to connect to the CloudflareWARP daemon\n", false},
		{"connected\n", true},
		{"disconnected\n", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := WarpStatusConnected(tc.raw); got != tc.want {
			t.Fatalf("WarpStatusConnected(%q)=%v want %v", tc.raw, got, tc.want)
		}
	}
}
