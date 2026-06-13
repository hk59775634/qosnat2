package lvs

import "testing"

func TestForwardFlag(t *testing.T) {
	if forwardFlag("nat") != "-m" {
		t.Fatal("nat should use -m")
	}
	if forwardFlag("dr") != "-g" {
		t.Fatal("dr should use -g")
	}
}
