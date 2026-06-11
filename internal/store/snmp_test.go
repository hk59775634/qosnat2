package store

import "testing"

func TestNormalizeSNMP(t *testing.T) {
	s := SNMPState{
		Enabled:     true,
		ROCommunity: "monitor",
		AllowedNetworks: []string{
			"192.168.1.0/24",
		},
	}
	if err := NormalizeSNMP(&s); err != nil {
		t.Fatal(err)
	}
	if s.Port != 161 {
		t.Fatalf("port=%d", s.Port)
	}
}

func TestNormalizeSNMPRequiresCommunity(t *testing.T) {
	s := SNMPState{Enabled: true}
	if err := NormalizeSNMP(&s); err == nil {
		t.Fatal("expected error")
	}
}
