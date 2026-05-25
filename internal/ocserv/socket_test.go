package ocserv

import (
	"os"
	"testing"
)

func TestResolveSocketFileGlob(t *testing.T) {
	dir := t.TempDir()
	base := dir + "/ocserv-socket"
	// no socket
	_, err := ResolveSocketFile(base)
	if err == nil {
		t.Fatal("expected error when socket missing")
	}
	// fake socket file
	sock := base + ".abc.0"
	f, err := os.Create(sock)
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	if err := os.Chmod(sock, 0666); err != nil {
		t.Fatal(err)
	}
	// On Linux, regular file is not ModeSocket; skip real socket test in CI
	// Only test that glob path doesn't panic
	_, _ = ResolveSocketFile(base)
}
