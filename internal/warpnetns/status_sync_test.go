package warpnetns

import "testing"

func TestRefreshConnectedStateNoNetns(t *testing.T) {
	if netnsExists() || linkExists(VethHost) {
		t.Skip("warp netns present on host")
	}
	if RefreshConnectedState() {
		t.Fatal("expected false without netns")
	}
}
