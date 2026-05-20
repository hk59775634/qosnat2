package conntrack

import "testing"

func TestParseLine(t *testing.T) {
	line := "tcp      6 299 ESTABLISHED src=10.0.0.1 dst=8.8.8.8 sport=12345 dport=53 src=8.8.8.8 dst=10.0.0.1 sport=53 dport=12345 [ASSURED] mark=0 use=1"
	e, ok := parseLine(line)
	if !ok {
		t.Fatal("parse failed")
	}
	if e.Protocol != "tcp" || e.Src != "10.0.0.1" || e.Dst != "8.8.8.8" || e.State != "ESTABLISHED" {
		t.Fatalf("unexpected: %+v", e)
	}
	if e.ReplySrc != "8.8.8.8" || e.ReplyDst != "10.0.0.1" {
		t.Fatalf("reply: %+v", e)
	}
}
