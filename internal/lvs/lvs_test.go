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

func TestIpvsProtoFlag(t *testing.T) {
	if ipvsProtoFlag("tcp") != "t" {
		t.Fatal("tcp")
	}
	if ipvsProtoFlag("udp") != "u" {
		t.Fatal("udp")
	}
	if ipvsProtoFlag("TCP") != "t" {
		t.Fatal("TCP")
	}
}
