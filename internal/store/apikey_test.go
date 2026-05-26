package store

import "testing"

func TestAPIKeyBcryptAndLegacy(t *testing.T) {
	plain := "qk_test_secret_key_12345"
	h, err := HashAPIKey(plain)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyAPIKey(plain, h) {
		t.Fatal("bcrypt verify failed")
	}
	legacy := LegacyAPIKeyHash(plain)
	if !VerifyAPIKey(plain, legacy) {
		t.Fatal("legacy verify failed")
	}
	if !IsLegacyAPIKeyHash(legacy) {
		t.Fatal("expected legacy marker")
	}
	if IsLegacyAPIKeyHash(h) {
		t.Fatal("bcrypt should not be legacy")
	}
}
