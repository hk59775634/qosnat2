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

func TestModuleLoaded(t *testing.T) {
	if !moduleLoaded("nf_conntrack") && !moduleLoaded("ip_vs") {
		t.Skip("no ip_vs or nf_conntrack loaded")
	}
}

func TestIpvsProtoFlag(t *testing.T) {
	if ipvsProtoFlag("tcp") != "t" || ipvsProtoFlag("udp") != "u" {
		t.Fatal("proto flag")
	}
}
