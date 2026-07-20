package dnsmasq

import (
	"os"
	"path/filepath"
	"strings"
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

func TestReplaceExecutableAtomic(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "dnsmasq")
	v1 := filepath.Join(dir, "v1")
	v2 := filepath.Join(dir, "v2")
	script := func(tag string) string {
		return "#!/bin/sh\nif [ \"$1\" = \"--help\" ]; then echo chnroutes-file " + tag + "; exit 0; fi\necho " + tag + "\nexit 0\n"
	}
	if err := os.WriteFile(v1, []byte(script("v1")), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(v2, []byte(script("v2")), 0755); err != nil {
		t.Fatal(err)
	}
	if err := replaceExecutableAtomic(v1, dst); err != nil {
		t.Fatal(err)
	}
	// Second replace must succeed even if dst already exists (same path as busy-file upgrade).
	if err := replaceExecutableAtomic(v2, dst); err != nil {
		t.Fatal(err)
	}
	if !BinarySupportsChnroutes(dst) {
		t.Fatal("replaced binary should support chnroutes")
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "v2") {
		t.Fatalf("expected v2 content, got %q", b)
	}
	if _, err := os.Stat(dst + ".new"); !os.IsNotExist(err) {
		t.Fatal("staging .new should be removed")
	}
	if _, err := os.Stat(dst + ".old"); !os.IsNotExist(err) {
		t.Fatal("backup .old should be removed")
	}
}
