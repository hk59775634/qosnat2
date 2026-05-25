package ocserv

import (
	"encoding/json"
	"testing"

	"github.com/hk59775634/qosnat2/internal/store"
)

func TestParseOcctlStatusJSON(t *testing.T) {
	raw := `{
    "Status": "online",
    "Active sessions": 2,
    "Total sessions": 10,
    "raw_rx": 1000,
    "raw_tx": 4000
  }`
	var st map[string]any
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatal(err)
	}
	if st["Status"] != "online" {
		t.Fatalf("status: %v", st["Status"])
	}
	if st["Active sessions"] != float64(2) {
		t.Fatalf("active: %v", st["Active sessions"])
	}
}

func TestParseOcctlUsersJSON(t *testing.T) {
	raw := `[
    {"Username": "alice", "ID": 1, "VPN IPv4": "10.250.0.2"}
  ]`
	var users []map[string]any
	if err := json.Unmarshal([]byte(raw), &users); err != nil {
		t.Fatal(err)
	}
	if len(users) != 1 || users[0]["Username"] != "alice" {
		t.Fatalf("%v", users)
	}
}

func TestOcctlFromStateDefaults(t *testing.T) {
	o := store.DefaultOCServ()
	c := OcctlFromState(o)
	if c.SocketFile != "/var/run/ocserv-socket" {
		t.Fatalf("socket: %s", c.SocketFile)
	}
	if c.UseOcctl {
		t.Fatal("default use_occtl should be false")
	}
}
