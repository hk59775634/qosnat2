package api

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSyncDevRolesFromFileOverridesStaleEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "env")
	old := defaultEnvPath
	defaultEnvPath = path
	t.Cleanup(func() { defaultEnvPath = old })

	_ = os.Setenv("DEV_LAN", "ens18")
	_ = os.Setenv("DEV_WAN", "ens19")
	if err := os.WriteFile(path, []byte("DEV_LAN=ens19\nDEV_WAN=ens18\n"), 0600); err != nil {
		t.Fatal(err)
	}

	lan, wan := syncDevRolesFromFile()
	if lan != "ens19" || wan != "ens18" {
		t.Fatalf("got lan=%q wan=%q, want ens19/ens18", lan, wan)
	}
	if os.Getenv("DEV_LAN") != "ens19" || os.Getenv("DEV_WAN") != "ens18" {
		t.Fatalf("env not updated: DEV_LAN=%q DEV_WAN=%q", os.Getenv("DEV_LAN"), os.Getenv("DEV_WAN"))
	}
}
