package netif

import (
	"os"
	"testing"
)

func TestIfIndexLoopback(t *testing.T) {
	if _, err := os.Stat("/sys/class/net/lo/ifindex"); err != nil {
		t.Skip("no /sys/class/net")
	}
	idx, err := IfIndex("lo")
	if err != nil || idx <= 0 {
		t.Fatalf("lo ifindex: idx=%d err=%v", idx, err)
	}
}

func TestIfIndexInvalidName(t *testing.T) {
	if _, err := IfIndex("../lo"); err == nil {
		t.Fatal("expected error for invalid name")
	}
}

func TestIfIndexMissing(t *testing.T) {
	if _, err := IfIndex("qosnat2-nonexistent-iface"); err == nil {
		t.Fatal("expected error for missing iface")
	}
}
