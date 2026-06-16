package sysctl

import "testing"

func TestValidateValue(t *testing.T) {
	if err := ValidateValue("134217728"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateValue("1\n2"); err == nil {
		t.Fatal("expected newline rejection")
	}
	if err := ValidateValue("a=b"); err == nil {
		t.Fatal("expected equals rejection")
	}
}
