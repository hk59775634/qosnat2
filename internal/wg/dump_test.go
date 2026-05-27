package wg

import (
	"testing"
	"time"
)

func TestParseShowDumpOutput(t *testing.T) {
	devLine := "privkey\tpubkey\t51820\toff\n"
	peerLine := "peerpub\t(none)\t(none)\t10.200.0.2/32\t1690000000\t0\t1234\t5678\toff\n"
	input := devLine + peerLine

	rows, err := ParseShowDumpOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 peer row, got %d", len(rows))
	}
	r := rows[0]
	if r.PublicKey != "peerpub" {
		t.Fatalf("pubkey: %q", r.PublicKey)
	}
	if r.RxBytes != 1234 || r.TxBytes != 5678 {
		t.Fatalf("rx/tx: %d %d", r.RxBytes, r.TxBytes)
	}
	if r.LastHandshake.Unix() != 1690000000 {
		t.Fatalf("handshake sec: %d", r.LastHandshake.Unix())
	}
}

func TestParseShowDumpOutputEightColumns(t *testing.T) {
	devLine := "privkey\tpubkey\t51820\toff\n"
	peerLine := "peerpub2\t(none)\t157.15.107.3:43879\t10.200.0.201/32\t1779890846\t4060764\t710419056\t25\n"
	input := devLine + peerLine

	rows, err := ParseShowDumpOutput(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("want 1 peer row, got %d", len(rows))
	}
	r := rows[0]
	if r.PublicKey != "peerpub2" {
		t.Fatalf("pubkey: %q", r.PublicKey)
	}
	if r.RxBytes != 4060764 || r.TxBytes != 710419056 {
		t.Fatalf("rx/tx: %d %d", r.RxBytes, r.TxBytes)
	}
	if r.LastHandshake.Unix() != 1779890846 {
		t.Fatalf("handshake sec: %d", r.LastHandshake.Unix())
	}
}

func TestPeerLikelyOnline(t *testing.T) {
	now := time.Unix(1700000000, 0)
	hs := now.Add(-2 * time.Minute)
	if !PeerLikelyOnline(hs, now, 10*time.Minute) {
		t.Fatal("expected online")
	}
	if PeerLikelyOnline(time.Time{}, now, 10*time.Minute) {
		t.Fatal("zero handshake should be offline")
	}
}
