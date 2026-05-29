package warpnetns

import "testing"

func TestNeedsNetnsResetNetnsWithoutVeth(t *testing.T) {
	if linkExists(VethHost) || netnsExists() {
		t.Skip("host has qwp0 or qosnat2-warp netns")
	}
	if needsNetnsReset() {
		t.Fatal("expected false with no netns and no veth")
	}
}

func TestRestoreHostResolvNoBackup(t *testing.T) {
	RestoreHostResolv() // must not panic
}

func TestWarpSvcStartArgsUsesUnshare(t *testing.T) {
	args := warpSvcStartArgs()
	if len(args) < 5 || args[0] != "netns" || args[3] != "unshare" {
		t.Fatalf("unexpected args: %v", args)
	}
}
