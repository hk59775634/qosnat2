package shaper

import "testing"

func TestEDTRootFQArgs(t *testing.T) {
	args := edtRootFQArgs("ens19", 1024, 1514)
	got := joinArgs(args)
	if contains(got, "codel") {
		t.Fatalf("edt root fq must not use fq_codel/codel: %v", args)
	}
	want := "tc qdisc add dev ens19 root fq flows 1024 quantum 1514"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func joinArgs(a []string) string {
	s := ""
	for i, v := range a {
		if i > 0 {
			s += " "
		}
		s += v
	}
	return s
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
