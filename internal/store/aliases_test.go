package store

import "testing"

func TestNormalizeAliasASN(t *testing.T) {
	a := &AliasSet{Name: "as13335", Type: "asn", ASN: 13335, Members: []string{"203.0.113.0/24"}}
	if err := NormalizeAlias(a); err != nil {
		t.Fatal(err)
	}
}
