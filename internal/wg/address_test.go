package wg

import (
	"testing"
)

func TestParseWGAddressList(t *testing.T) {
	got, err := parseWGAddressList("10.200.0.1/24, fd00::1/64")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"10.200.0.1/24", "fd00::1/64"}
	if !cidrSetsEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}

	got, err = parseWGAddressList("100.64.0.1")
	if err != nil {
		t.Fatal(err)
	}
	if !cidrSetsEqual(got, []string{"100.64.0.1/32"}) {
		t.Fatalf("bare IP: got %v", got)
	}

	got, err = parseWGAddressList("")
	if err != nil || len(got) != 0 {
		t.Fatalf("empty: got %v err %v", got, err)
	}
}

func TestCidrSetsEqual_DetectsAddressChange(t *testing.T) {
	oldA, _ := parseWGAddressList("10.200.0.1/24")
	newA, _ := parseWGAddressList("10.200.0.2/24")
	if cidrSetsEqual(oldA, newA) {
		t.Fatal("different addresses should not equal")
	}
	same, _ := parseWGAddressList(" 10.200.0.1/24 ")
	if !cidrSetsEqual(oldA, same) {
		t.Fatal("normalized equal addresses should match")
	}
}
