package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadRecoversFromBakOnCorruptMain(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	good := []byte(`{"setup_complete":true,"admin_user":"admin"}`)
	if err := WriteFileAtomic(path, good, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path+".bak", good, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{not json"), 0600); err != nil {
		t.Fatal(err)
	}
	st := New(path)
	if err := st.Load(); err != nil {
		t.Fatal(err)
	}
	if !st.Get().SetupComplete {
		t.Fatal("expected state from .bak")
	}
	main, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(main) != string(good) {
		t.Fatalf("main file not restored from backup: %q", main)
	}
}

func TestLoadFromBakWhenMainMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	good := []byte(`{"admin_user":"from-bak"}`)
	if err := os.WriteFile(path+".bak", good, 0600); err != nil {
		t.Fatal(err)
	}
	st := New(path)
	if err := st.Load(); err != nil {
		t.Fatal(err)
	}
	if st.Get().AdminUser != "from-bak" {
		t.Fatalf("got admin_user %q", st.Get().AdminUser)
	}
}
