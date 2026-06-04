package dnsmasq

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBinarySupportsChnroutes(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "fake-dnsmasq")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then echo chnroutes-file; exit 0; fi\nexit 0\n"
	if err := os.WriteFile(p, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	if !BinarySupportsChnroutes(p) {
		t.Fatal("expected chnroutes support")
	}
}

func TestInstallChnroutesBinary(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip("requires root for /usr paths")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "dnsmasq")
	script := "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then echo chnroutes-file; exit 0; fi\nexit 0\n"
	if err := os.WriteFile(src, []byte(script), 0755); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(dir, "installed")
	if err := copyExecutable(src, dst); err != nil {
		t.Fatal(err)
	}
	if !BinarySupportsChnroutes(dst) {
		t.Fatal("installed copy should support chnroutes")
	}
}
