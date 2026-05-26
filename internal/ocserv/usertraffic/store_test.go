package usertraffic

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestTotalBytes(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "traffic.json"))
	now := time.Now().Truncate(time.Minute)
	_ = s.RecordCounters("bob", 100, 50, now)
	_ = s.RecordCounters("bob", 300, 150, now.Add(time.Hour))
	rx, tx := s.TotalBytes("bob")
	if rx != 300 || tx != 150 {
		t.Fatalf("total: rx=%d tx=%d", rx, tx)
	}
}

func TestRecordAndPrune(t *testing.T) {
	dir := t.TempDir()
	s := NewStore(filepath.Join(dir, "traffic.json"))
	now := time.Now().Truncate(time.Minute)
	if err := s.RecordCounters("alice", 1000, 500, now); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordCounters("alice", 2000, 800, now.Add(5*time.Minute)); err != nil {
		t.Fatal(err)
	}
	resp := s.Query("alice", "24h")
	if resp.Summary.PeriodRxBytes == 0 && resp.Summary.PeriodTxBytes == 0 {
		t.Fatalf("expected period bytes, got %+v", resp.Summary)
	}
	old := time.Now().Add(-400 * 24 * time.Hour)
	s.mu.Lock()
	u := s.data.Users["alice"]
	u.Hourly = append(u.Hourly, Bucket{Ts: old.Unix(), RxBytes: 1, TxBytes: 1})
	s.mu.Unlock()
	_ = s.RecordCounters("alice", 2000, 800, now)
	s.mu.Lock()
	for _, b := range s.data.Users["alice"].Hourly {
		if b.Ts == old.Unix() {
			t.Fatal("bucket older than 1y should be pruned")
		}
	}
	s.mu.Unlock()
	_ = os.Remove(s.path)
}
