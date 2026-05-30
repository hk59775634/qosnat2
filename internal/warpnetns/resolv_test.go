package warpnetns

import (
	"bytes"
	"os"
	"testing"
)

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

func TestEnsureNetnsResolvFileUsesCloudflareDNS(t *testing.T) {
	_ = os.MkdirAll(netnsResolvDir, 0755)
	_ = os.WriteFile(netnsResolvFile, []byte("nameserver 180.76.76.76\n"), 0644)
	ensureNetnsResolvFile()
	b, err := os.ReadFile(netnsResolvFile)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("1.1.1.1")) {
		t.Fatalf("expected Cloudflare DNS, got %q", b)
	}
}
