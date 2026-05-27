package usertraffic

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStoreRecordAndQuery(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "wg-traffic.json"))
	now := time.Now().Truncate(time.Minute)
	if err := s.RecordCounters("p1", 1000, 2000, now); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCounters("p1", 1500, 2800, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	rx, tx := s.TotalBytes("p1")
	if rx == 0 && tx == 0 {
		t.Fatal("expected non-zero totals")
	}
	q := s.Query("p1", "7d")
	if q.Peer != "p1" {
		t.Fatalf("peer %q", q.Peer)
	}
	if len(q.Series) == 0 {
		t.Fatal("expected series points")
	}
}
