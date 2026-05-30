package releasecatalog

import (
	"os"
	"path/filepath"
	"testing"
)

func TestReplaceFileAtomic(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "qosnatd")
	srcNew := filepath.Join(dir, "qosnatd-next")
	if err := os.WriteFile(dst, []byte("v1"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcNew, []byte("v2"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := replaceFileAtomic(srcNew, dst, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "v2" {
		t.Fatalf("want v2, got %q", b)
	}
	if _, err := os.Stat(dst + ".new"); err == nil {
		t.Fatal("staging file should be gone")
	}
}

func TestReplaceFileAtomicWhileOpen(t *testing.T) {
	dir := t.TempDir()
	dst := filepath.Join(dir, "busybin")
	srcNew := filepath.Join(dir, "busybin-next")
	if err := os.WriteFile(dst, []byte("old"), 0755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := os.WriteFile(srcNew, []byte("new"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := replaceFileAtomic(srcNew, dst, 0755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "new" {
		t.Fatalf("path should point to new content, got %q", b)
	}
}
