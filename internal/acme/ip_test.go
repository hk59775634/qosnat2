package acme

import "testing"

func TestNormalizeIP(t *testing.T) {
	got, err := NormalizeIP(" 203.0.113.10 ")
	if err != nil || got != "203.0.113.10" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := NormalizeIP("not-an-ip"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := NormalizeIP("2001:db8::1"); err == nil {
		t.Fatal("expected ipv4 only error")
	}
}
