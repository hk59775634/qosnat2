package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")
	data := []byte(`{"ok":true}`)
	if err := WriteFileAtomic(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(data) {
		t.Fatalf("got %q want %q", got, data)
	}
}

func TestCloneNatIPv4(t *testing.T) {
	src := NatIPv4State{
		PolicyRoutes:   []string{"10.0.0.0/8"},
		SharedIPs:      []string{"1.2.3.4"},
		StaticMappings: map[string]string{"10.0.0.1": "203.0.113.1"},
		PrefixMappings: map[string]string{"10.0.0.0/24": "203.0.113.0/24"},
	}
	cl := CloneNatIPv4(src)
	cl.StaticMappings["10.0.0.1"] = "changed"
	cl.PolicyRoutes[0] = "changed"
	if src.StaticMappings["10.0.0.1"] != "203.0.113.1" {
		t.Fatal("clone must not alias maps")
	}
	if src.PolicyRoutes[0] != "10.0.0.0/8" {
		t.Fatal("clone must not alias slices")
	}
}
